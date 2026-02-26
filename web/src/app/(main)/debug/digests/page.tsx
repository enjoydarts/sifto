"use client";

import { FormEvent, useMemo, useState } from "react";
import { api, DigestDetail } from "@/lib/api";
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
  const [busySystemHealth, setBusySystemHealth] = useState(false);
  const [systemHealth, setSystemHealth] = useState<DebugSystemStatusResponse | null>(null);
  const [webHealth, setWebHealth] = useState<{ status: number; latency_ms: number; body: unknown } | null>(null);

  const helperText = useMemo(
    () =>
      "digest_date は JST の日付です。例: 2026-02-24 を指定すると、2026-02-23 JST に生成された要約が対象になります。",
    []
  );

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
            {busySystemHealth ? "読み込み中…" : "Refresh"}
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
              <p className="text-xs text-zinc-400">未取得</p>
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
                {Object.keys(systemHealth.data?.cache_stats ?? {}).length > 0 && (
                  <div className="pt-1">
                    <div className="mb-2 text-[11px] font-medium text-zinc-600">Cache Stats</div>
                    <div className="grid gap-2 sm:grid-cols-2">
                      {Object.entries(systemHealth.data?.cache_stats ?? {}).map(([name, stat]) => (
                        <div key={name} className="rounded border border-zinc-200 px-2 py-2">
                          <div className="mb-1 text-[11px] font-medium text-zinc-800">{name}</div>
                          <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-[11px] text-zinc-600">
                            <span>hit</span>
                            <span className="text-right">{stat.hits ?? 0}</span>
                            <span>miss</span>
                            <span className="text-right">{stat.misses ?? 0}</span>
                            <span>bypass</span>
                            <span className="text-right">{stat.bypass ?? 0}</span>
                            <span>errors</span>
                            <span className="text-right">{stat.errors ?? 0}</span>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            ) : (
              <p className="text-xs text-zinc-400">未取得</p>
            )}
          </div>
        </div>
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
            `skip_send`（作成だけして `digest/created` は送らない）
          </label>
          <button
            type="submit"
            disabled={busyGenerate}
            className="rounded bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700 disabled:opacity-50"
          >
            {busyGenerate ? "実行中…" : "Generate Debug"}
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
            {busySend ? "実行中…" : "Queue Send"}
          </button>
          <button
            type="button"
            disabled={busyInspect || !digestId.trim()}
            onClick={inspectDigest}
            className="rounded border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-50"
          >
            {busyInspect ? "確認中…" : "状態確認"}
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
            `dry_run`（候補確認だけしてキュー投入しない）
          </label>
          <button
            type="submit"
            disabled={busyBackfill}
            className="rounded bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700 disabled:opacity-50"
          >
            {busyBackfill ? "実行中…" : backfillDryRun ? "Preview Backfill" : "Queue Backfill"}
          </button>
        </form>

        {backfillResult && (
          <pre className="mt-4 overflow-x-auto rounded bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(backfillResult, null, 2)}
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
