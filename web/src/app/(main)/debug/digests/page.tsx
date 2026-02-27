"use client";

import { FormEvent, useMemo, useState } from "react";
import { api, BulkRetryFailedResult, DigestDetail } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";

type GenerateResponse = {
  status: string;
  digest_date: string;
  digests_created: number;
  events_enqueued: number;
  skipped_no_items: number;
  skipped_sent: number;
  errors: number;
  users_checked: number;
  results?: unknown;
};

type SendResponse = {
  status: string;
  digest_id: string;
  user_id: string;
  to: string;
};

type EmbeddingBackfillResponse = {
  status: string;
  dry_run: boolean;
  user_filter?: string | null;
  limit: number;
  matched: number;
  queued_count: number;
  failed_count: number;
  send_error_samples?: unknown[];
  targets?: unknown[];
};

type DebugSystemStatusResponse = {
  proxy_status: number;
  proxy_latency_ms: number;
  data?: {
    status?: string;
    checked_at?: string;
    checks?: Record<
      string,
      {
        status?: string;
        latency_ms?: number;
        detail?: string;
        http_status?: number;
        meta?: Record<string, unknown>;
      }
    >;
    cache_stats?: Record<
      string,
      {
        hits?: number;
        misses?: number;
        bypass?: number;
        errors?: number;
      }
    >;
    cache_stats_by_window?: Record<
      string,
      {
        dashboard?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        reading_plan?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        error?: string;
      }
    >;
  };
};

async function postJSON<T>(url: string, body: unknown): Promise<T> {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`${res.status}: ${text || res.statusText}`);
  }
  return text ? (JSON.parse(text) as T) : ({} as T);
}

function todayJstPlusOneDateString() {
  const now = new Date();
  const jst = new Date(now.toLocaleString("en-US", { timeZone: "Asia/Tokyo" }));
  jst.setDate(jst.getDate() + 1);
  const y = jst.getFullYear();
  const m = String(jst.getMonth() + 1).padStart(2, "0");
  const d = String(jst.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

export default function DebugDigestsPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const [userId, setUserId] = useState("00000000-0000-0000-0000-000000000001");
  const [digestDate, setDigestDate] = useState(todayJstPlusOneDateString());
  const [skipSend, setSkipSend] = useState(true);
  const [digestId, setDigestId] = useState("");

  const [busyGenerate, setBusyGenerate] = useState(false);
  const [busySend, setBusySend] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [generateResult, setGenerateResult] = useState<GenerateResponse | null>(null);
  const [sendResult, setSendResult] = useState<SendResponse | null>(null);
  const [digestDetail, setDigestDetail] = useState<DigestDetail | null>(null);
  const [busyInspect, setBusyInspect] = useState(false);
  const [backfillUserId, setBackfillUserId] = useState("");
  const [backfillLimit, setBackfillLimit] = useState("100");
  const [backfillDryRun, setBackfillDryRun] = useState(true);
  const [busyBackfill, setBusyBackfill] = useState(false);
  const [backfillResult, setBackfillResult] = useState<EmbeddingBackfillResponse | null>(null);
  const [retrySourceId, setRetrySourceId] = useState("");
  const [busyRetryFailed, setBusyRetryFailed] = useState(false);
  const [retryFailedResult, setRetryFailedResult] = useState<BulkRetryFailedResult | null>(null);
  const [busySystemHealth, setBusySystemHealth] = useState(false);
  const [systemHealth, setSystemHealth] = useState<DebugSystemStatusResponse | null>(null);
  const [webHealth, setWebHealth] = useState<{ status: number; latency_ms: number; body: unknown } | null>(null);

  const helperText = useMemo(
    () =>
      t("debug.digest.helperText"),
    [t]
  );
  const cacheWindowRows = useMemo(() => {
    const rows = systemHealth?.data?.cache_stats_by_window ?? {};
    const order = ["1h", "3h", "8h", "24h", "3d", "7d"];
    return order
      .filter((k) => rows[k])
      .map((k) => ({
        window: k,
        dashboard_hit_rate:
          typeof rows[k]?.dashboard?.hit_rate === "number"
            ? Number((rows[k]!.dashboard!.hit_rate! * 100).toFixed(1))
            : null,
        reading_plan_hit_rate:
          typeof rows[k]?.reading_plan?.hit_rate === "number"
            ? Number((rows[k]!.reading_plan!.hit_rate! * 100).toFixed(1))
            : null,
        dashboard_hits: rows[k]?.dashboard?.hits ?? 0,
        dashboard_misses: rows[k]?.dashboard?.misses ?? 0,
        reading_plan_hits: rows[k]?.reading_plan?.hits ?? 0,
        reading_plan_misses: rows[k]?.reading_plan?.misses ?? 0,
      }));
  }, [systemHealth]);

  const onGenerate = async (e: FormEvent) => {
    e.preventDefault();
    setBusyGenerate(true);
    setError(null);
    setGenerateResult(null);
    try {
      const payload: { user_id?: string; digest_date?: string; skip_send: boolean } = {
        skip_send: skipSend,
      };
      if (userId.trim()) payload.user_id = userId.trim();
      if (digestDate.trim()) payload.digest_date = digestDate.trim();
      const res = await postJSON<GenerateResponse>("/api/debug/digests/generate", payload);
      setGenerateResult(res);
      showToast("Generate debug request queued", "success");
      const firstDigestId =
        Array.isArray((res as { results?: unknown[] }).results) &&
        (res as { results?: Array<{ digest_id?: string }> }).results?.[0]?.digest_id;
      if (typeof firstDigestId === "string" && firstDigestId) {
        setDigestId(firstDigestId);
      }
    } catch (e) {
      setError(String(e));
      showToast(String(e), "error");
    } finally {
      setBusyGenerate(false);
    }
  };

  const onSend = async (e: FormEvent) => {
    e.preventDefault();
    if (!digestId.trim()) return;
    setBusySend(true);
    setError(null);
    setSendResult(null);
    try {
      const res = await postJSON<SendResponse>("/api/debug/digests/send", {
        digest_id: digestId.trim(),
      });
      setSendResult(res);
      showToast("Send debug request queued", "success");
    } catch (e) {
      setError(String(e));
      showToast(String(e), "error");
    } finally {
      setBusySend(false);
    }
  };

  const inspectDigest = async () => {
    if (!digestId.trim()) return;
    setBusyInspect(true);
    setError(null);
    try {
      const detail = await api.getDigest(digestId.trim());
      setDigestDetail(detail);
      showToast("Digest status loaded", "info");
    } catch (e) {
      setError(String(e));
      showToast(String(e), "error");
    } finally {
      setBusyInspect(false);
    }
  };

  const onBackfillEmbeddings = async (e: FormEvent) => {
    e.preventDefault();
    setBusyBackfill(true);
    setError(null);
    setBackfillResult(null);
    try {
      const parsedLimit = Number(backfillLimit);
      if (!Number.isFinite(parsedLimit) || parsedLimit < 1 || parsedLimit > 1000) {
        throw new Error("limit must be between 1 and 1000");
      }
      const payload: { user_id?: string; limit: number; dry_run: boolean } = {
        limit: parsedLimit,
        dry_run: backfillDryRun,
      };
      if (backfillUserId.trim()) payload.user_id = backfillUserId.trim();
      const res = await postJSON<EmbeddingBackfillResponse>("/api/debug/embeddings/backfill", payload);
      setBackfillResult(res);
      showToast(
        backfillDryRun ? "Embedding backfill dry-run completed" : "Embedding backfill queued",
        "success"
      );
    } catch (e) {
      setError(String(e));
      showToast(String(e), "error");
    } finally {
      setBusyBackfill(false);
    }
  };

  const onRetryFailedItems = async (e: FormEvent) => {
    e.preventDefault();
    setBusyRetryFailed(true);
    setError(null);
    setRetryFailedResult(null);
    try {
      const res = await api.retryFailedItems(
        retrySourceId.trim() ? { source_id: retrySourceId.trim() } : undefined
      );
      setRetryFailedResult(res);
      showToast("Failed items retry queued", "success");
    } catch (e) {
      setError(String(e));
      showToast(String(e), "error");
    } finally {
      setBusyRetryFailed(false);
    }
  };

  const loadSystemHealth = async () => {
    setBusySystemHealth(true);
    setError(null);
    try {
      const webStart = Date.now();
      const webRes = await fetch("/health", { cache: "no-store" });
      const webText = await webRes.text();
      let webBody: unknown = null;
      try {
        webBody = webText ? JSON.parse(webText) : null;
      } catch {
        webBody = { raw: webText };
      }
      setWebHealth({
        status: webRes.status,
        latency_ms: Date.now() - webStart,
        body: webBody,
      });

      const res = await fetch("/api/debug/system-status", { cache: "no-store" });
      const text = await res.text();
      if (!res.ok) {
        throw new Error(`${res.status}: ${text || res.statusText}`);
      }
      setSystemHealth(text ? (JSON.parse(text) as DebugSystemStatusResponse) : null);
      showToast("System health loaded", "info");
    } catch (e) {
      setError(String(e));
      showToast(String(e), "error");
    } finally {
      setBusySystemHealth(false);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Debug Digests</h1>
        <p className="mt-2 text-sm text-zinc-500">{helperText}</p>
      </div>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-zinc-800">System Health (Debug)</h2>
          <button
            type="button"
            onClick={loadSystemHealth}
            disabled={busySystemHealth}
            className="rounded border border-zinc-300 bg-white px-3 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-50"
          >
            {busySystemHealth ? t("common.loading") : t("common.refresh")}
          </button>
        </div>
        <div className="grid gap-4 lg:grid-cols-2">
          <div className="rounded border border-zinc-200 p-3">
            <div className="mb-2 text-xs font-medium text-zinc-600">Web /health</div>
            {webHealth ? (
              <div className="space-y-2 text-xs">
                <div className="flex items-center gap-2">
                  <StatusPill ok={webHealth.status >= 200 && webHealth.status < 400} label={`HTTP ${webHealth.status}`} />
                  <span className="text-zinc-500">{webHealth.latency_ms} ms</span>
                </div>
                <pre className="overflow-x-auto rounded bg-zinc-950 p-2 text-[11px] text-zinc-100">
                  {JSON.stringify(webHealth.body, null, 2)}
                </pre>
              </div>
            ) : (
              <p className="text-xs text-zinc-400">{t("debug.notFetched")}</p>
            )}
          </div>
          <div className="rounded border border-zinc-200 p-3">
            <div className="mb-2 text-xs font-medium text-zinc-600">API Internal System Status</div>
            {systemHealth ? (
              <div className="space-y-3 text-xs">
                <div className="flex items-center gap-2">
                  <StatusPill ok={systemHealth.data?.status === "ok"} label={systemHealth.data?.status ?? "unknown"} />
                  <span className="text-zinc-500">proxy {systemHealth.proxy_latency_ms} ms</span>
                </div>
                <div className="space-y-2">
                  {Object.entries(systemHealth.data?.checks ?? {}).map(([name, row]) => (
                    <div key={name} className="rounded border border-zinc-200 px-2 py-2">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div className="font-medium text-zinc-800">{name}</div>
                        <div className="flex items-center gap-2">
                          <StatusPill ok={row.status === "ok"} label={row.status ?? "unknown"} />
                          {typeof row.latency_ms === "number" && (
                            <span className="text-zinc-500">{row.latency_ms} ms</span>
                          )}
                          {typeof row.http_status === "number" && row.http_status > 0 && (
                            <span className="text-zinc-500">HTTP {row.http_status}</span>
                          )}
                        </div>
                      </div>
                      {row.detail && <div className="mt-1 text-zinc-500">{row.detail}</div>}
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <p className="text-xs text-zinc-400">{t("debug.notFetched")}</p>
            )}
          </div>
        </div>
      </section>

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <h2 className="mb-3 text-sm font-semibold text-zinc-800">Cache Hit Rate (Debug)</h2>
        {!systemHealth ? (
          <p className="text-xs text-zinc-400">{t("debug.systemHealthFirst")}</p>
        ) : (
          <div className="space-y-4">
            <div className="rounded border border-zinc-200 p-3">
              <div className="mb-2 text-xs font-medium text-zinc-600">Current Process Counters</div>
              <div className="grid gap-2 sm:grid-cols-2">
                {Object.entries(systemHealth.data?.cache_stats ?? {}).map(([name, stat]) => (
                  <div key={name} className="rounded border border-zinc-200 px-2 py-2">
                    <div className="mb-1 text-[11px] font-medium text-zinc-800">{name}</div>
                    <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-[11px] text-zinc-600">
                      <span>hit</span><span className="text-right">{stat.hits ?? 0}</span>
                      <span>miss</span><span className="text-right">{stat.misses ?? 0}</span>
                      <span>bypass</span><span className="text-right">{stat.bypass ?? 0}</span>
                      <span>errors</span><span className="text-right">{stat.errors ?? 0}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {cacheWindowRows.length > 0 && (
              <div className="rounded border border-zinc-200 p-3">
                <div className="mb-2 text-xs font-medium text-zinc-600">Windowed Hit Rate</div>
                <div className="overflow-x-auto">
                  <table className="min-w-full border-separate border-spacing-0 text-[11px]">
                    <thead>
                      <tr className="text-zinc-500">
                        <th className="border-b border-zinc-200 px-2 py-1 text-left font-medium">Window</th>
                        <th className="border-b border-zinc-200 px-2 py-1 text-left font-medium">dashboard</th>
                        <th className="border-b border-zinc-200 px-2 py-1 text-right font-medium">h/m</th>
                        <th className="border-b border-zinc-200 px-2 py-1 text-left font-medium">reading_plan</th>
                        <th className="border-b border-zinc-200 px-2 py-1 text-right font-medium">h/m</th>
                      </tr>
                    </thead>
                    <tbody>
                      {cacheWindowRows.map((row) => (
                        <tr key={row.window}>
                          <td className="border-b border-zinc-100 px-2 py-1.5 font-medium text-zinc-700">
                            {row.window}
                          </td>
                          <td className="border-b border-zinc-100 px-2 py-1.5">
                            <RateCell value={row.dashboard_hit_rate} />
                          </td>
                          <td className="border-b border-zinc-100 px-2 py-1.5 text-right text-zinc-500">
                            {row.dashboard_hits}/{row.dashboard_misses}
                          </td>
                          <td className="border-b border-zinc-100 px-2 py-1.5">
                            <RateCell value={row.reading_plan_hit_rate} />
                          </td>
                          <td className="border-b border-zinc-100 px-2 py-1.5 text-right text-zinc-500">
                            {row.reading_plan_hits}/{row.reading_plan_misses}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {Object.keys(systemHealth.data?.cache_stats_by_window ?? {}).length > 0 && (
              <div className="space-y-2">
                {Object.entries(systemHealth.data?.cache_stats_by_window ?? {}).map(([win, row]) => (
                  <div key={win} className="rounded border border-zinc-200 px-2 py-2">
                    <div className="mb-1 text-[11px] font-medium text-zinc-800">{win}</div>
                    {"error" in row && row.error ? (
                      <div className="text-[11px] text-red-600">{row.error}</div>
                    ) : (
                      <div className="grid gap-2 sm:grid-cols-2">
                        {(["dashboard", "reading_plan"] as const).map((k) => {
                          const v = row[k];
                          const rate = typeof v?.hit_rate === "number" ? `${(v.hit_rate * 100).toFixed(1)}%` : "N/A";
                          return (
                            <div key={k} className="rounded bg-zinc-50 px-2 py-1.5">
                              <div className="flex items-center justify-between text-[11px]">
                                <span className="font-medium text-zinc-700">{k}</span>
                                <span className="text-zinc-900">{rate}</span>
                              </div>
                              <div className="mt-0.5 text-[10px] text-zinc-500">
                                h {v?.hits ?? 0} / m {v?.misses ?? 0} / b {v?.bypass ?? 0} / e {v?.errors ?? 0}
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </section>

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <h2 className="mb-3 text-sm font-semibold text-zinc-800">Generate Digest (Debug)</h2>
        <form onSubmit={onGenerate} className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-zinc-600">User ID (optional)</div>
              <input
                value={userId}
                onChange={(e) => setUserId(e.target.value)}
                className="w-full rounded border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
                placeholder="all users if empty"
              />
            </label>
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-zinc-600">Digest Date (JST)</div>
              <input
                type="date"
                value={digestDate}
                onChange={(e) => setDigestDate(e.target.value)}
                className="w-full rounded border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
              />
            </label>
          </div>
          <label className="flex items-center gap-2 text-sm text-zinc-700">
            <input
              type="checkbox"
              checked={skipSend}
              onChange={(e) => setSkipSend(e.target.checked)}
              className="accent-zinc-900"
            />
            {t("debug.digest.skipSend")}
          </label>
          <button
            type="submit"
            disabled={busyGenerate}
            className="rounded bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700 disabled:opacity-50"
          >
            {busyGenerate ? t("debug.running") : "Generate Debug"}
          </button>
        </form>

        {generateResult && (
          <pre className="mt-4 overflow-x-auto rounded bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(generateResult, null, 2)}
          </pre>
        )}
      </section>

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <h2 className="mb-3 text-sm font-semibold text-zinc-800">Send Digest (Debug)</h2>
        <form onSubmit={onSend} className="flex flex-col gap-3 sm:flex-row sm:items-end">
          <label className="flex-1 text-sm">
            <div className="mb-1 text-xs font-medium text-zinc-600">Digest ID</div>
            <input
              value={digestId}
              onChange={(e) => setDigestId(e.target.value)}
              className="w-full rounded border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
              placeholder="00000000-0000-0000-0000-0000000000dd"
            />
          </label>
          <button
            type="submit"
            disabled={busySend || !digestId.trim()}
            className="rounded bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700 disabled:opacity-50"
          >
            {busySend ? t("debug.running") : "Queue Send"}
          </button>
          <button
            type="button"
            disabled={busyInspect || !digestId.trim()}
            onClick={inspectDigest}
            className="rounded border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-50"
          >
            {busyInspect ? t("debug.inspecting") : t("debug.statusCheck")}
          </button>
        </form>

        {sendResult && (
          <pre className="mt-4 overflow-x-auto rounded bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(sendResult, null, 2)}
          </pre>
        )}

        {digestDetail && (
          <div className="mt-4 rounded border border-zinc-200 bg-zinc-50 p-3 text-sm">
            <div><span className="font-medium">digest_id:</span> {digestDetail.id}</div>
            <div><span className="font-medium">send_status:</span> {digestDetail.send_status ?? "-"}</div>
            <div><span className="font-medium">send_tried_at:</span> {digestDetail.send_tried_at ?? "-"}</div>
            <div><span className="font-medium">sent_at:</span> {digestDetail.sent_at ?? "-"}</div>
            <div><span className="font-medium">email_copy:</span> {digestDetail.email_subject && digestDetail.email_body ? "generated" : "not generated"}</div>
            {digestDetail.send_error && (
              <pre className="mt-2 overflow-x-auto rounded bg-zinc-950 p-3 text-xs text-zinc-100">
                {digestDetail.send_error}
              </pre>
            )}
          </div>
        )}
      </section>

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <h2 className="mb-3 text-sm font-semibold text-zinc-800">Embeddings Backfill (Debug)</h2>
        <form onSubmit={onBackfillEmbeddings} className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <label className="text-sm sm:col-span-2">
              <div className="mb-1 text-xs font-medium text-zinc-600">User ID (optional)</div>
              <input
                value={backfillUserId}
                onChange={(e) => setBackfillUserId(e.target.value)}
                className="w-full rounded border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
                placeholder="all users if empty"
              />
            </label>
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-zinc-600">Limit (1-1000)</div>
              <input
                type="number"
                min={1}
                max={1000}
                value={backfillLimit}
                onChange={(e) => setBackfillLimit(e.target.value)}
                className="w-full rounded border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
              />
            </label>
          </div>
          <label className="flex items-center gap-2 text-sm text-zinc-700">
            <input
              type="checkbox"
              checked={backfillDryRun}
              onChange={(e) => setBackfillDryRun(e.target.checked)}
              className="accent-zinc-900"
            />
            {t("debug.digest.dryRun")}
          </label>
          <button
            type="submit"
            disabled={busyBackfill}
            className="rounded bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700 disabled:opacity-50"
          >
            {busyBackfill ? t("debug.running") : backfillDryRun ? "Preview Backfill" : "Queue Backfill"}
          </button>
        </form>

        {backfillResult && (
          <pre className="mt-4 overflow-x-auto rounded bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(backfillResult, null, 2)}
          </pre>
        )}
      </section>

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <h2 className="mb-3 text-sm font-semibold text-zinc-800">Retry Failed Items (Debug)</h2>
        <form onSubmit={onRetryFailedItems} className="space-y-4">
          <label className="block text-sm">
            <div className="mb-1 text-xs font-medium text-zinc-600">Source ID (optional)</div>
            <input
              value={retrySourceId}
              onChange={(e) => setRetrySourceId(e.target.value)}
              className="w-full rounded border border-zinc-300 px-3 py-2 text-sm outline-none focus:border-zinc-500"
              placeholder="all failed items if empty"
            />
          </label>
          <div className="pt-1">
            <button
              type="submit"
              disabled={busyRetryFailed}
              className="rounded bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700 disabled:opacity-50"
            >
              {busyRetryFailed ? t("debug.running") : "Queue Retry Failed"}
            </button>
          </div>
        </form>

        {retryFailedResult && (
          <pre className="mt-4 overflow-x-auto rounded bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(retryFailedResult, null, 2)}
          </pre>
        )}
      </section>
    </div>
  );
}

function StatusPill({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span
      className={`rounded px-2 py-0.5 text-[11px] font-medium ${
        ok ? "bg-green-50 text-green-700" : "bg-red-50 text-red-700"
      }`}
    >
      {label}
    </span>
  );
}

function RateCell({ value }: { value: number | null }) {
  if (typeof value !== "number") {
    return <span className="text-zinc-400">N/A</span>;
  }
  let tone = "bg-zinc-100 text-zinc-700";
  if (value >= 90) tone = "bg-green-100 text-green-800";
  else if (value >= 70) tone = "bg-emerald-50 text-emerald-700";
  else if (value >= 50) tone = "bg-amber-50 text-amber-700";
  else tone = "bg-red-50 text-red-700";
  return (
    <span className={`inline-flex min-w-14 justify-center rounded px-2 py-0.5 font-medium ${tone}`}>
      {value.toFixed(1)}%
    </span>
  );
}
