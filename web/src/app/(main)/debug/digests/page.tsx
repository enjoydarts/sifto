"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useUser } from "@clerk/nextjs";
import { Bug } from "lucide-react";
import { api, BulkRetryFailedResult, DigestDetail } from "@/lib/api";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";
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

type TitleBackfillResponse = {
  status: string;
  dry_run: boolean;
  user_filter?: string | null;
  limit: number;
  matched: number;
  updated_count: number;
  empty_count: number;
  failed_count: number;
  error_samples?: unknown[];
  targets?: unknown[];
};

type OpenRouterCostBackfillResponse = {
  status: string;
  dry_run: boolean;
  user_filter?: string | null;
  limit: number;
  from?: string | null;
  to?: string | null;
  matched: number;
  repaired: number;
  zeroed: number;
  failed: number;
  error_samples?: unknown[];
  targets?: unknown[];
};

type SearchBackfillResponse = {
  ok: boolean;
  run_id: string;
  offset: number;
  limit: number;
  all: boolean;
  total_items: number;
  queued_batches: number;
};

type SearchBackfillRun = {
  id: string;
  requested_offset: number;
  batch_size: number;
  all_items: boolean;
  total_items: number;
  queued_batches: number;
  completed_batches: number;
  failed_batches: number;
  processed_items: number;
  status: string;
  last_error?: string | null;
  created_at: string;
  updated_at: string;
  started_at?: string | null;
  finished_at?: string | null;
};

type SearchBackfillRunsResponse = {
  runs: SearchBackfillRun[];
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
        items_list?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        focus_queue?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        triage_all?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        related?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        briefing?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        error?: string;
      }
    >;
    cache_metrics_user_id?: string;
    cache_stats_by_window_user?: Record<
      string,
      {
        dashboard?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        reading_plan?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        items_list?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        focus_queue?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        triage_all?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        related?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        briefing?: { hits?: number; misses?: number; bypass?: number; errors?: number; hit_rate?: number | null };
        error?: string;
      }
    >;
  };
};

type OneSignalDebugState = {
  checked_at: string;
  app_id_configured: boolean;
  app_id_preview: string | null;
  script_loading: boolean;
  script_loaded: boolean;
  script_error: string | null;
  deferred_executed: boolean;
  init_enqueued_count: number;
  sdk_ready: boolean;
  sdk_loaded: boolean;
  deferred_queue_length: number;
  init_error: string | null;
  permission: string;
  opted_in: boolean | null;
  login_external_id: string | null;
  subscription_id: string | null;
  sw_scopes: string[];
  sw_registrations: Array<{ scope: string; script_url: string | null }>;
  worker_file_reachable: boolean | null;
  worker_updater_reachable: boolean | null;
};

type PushTestResponse = {
  status: string;
  target?: string;
  external_id?: string;
  subscription_id?: string;
  title: string;
  message: string;
  result?: {
    id?: string;
    recipients?: number;
  };
};

type DebugSection = "system" | "digestOps" | "backfills" | "recovery";

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

function searchBackfillProgress(run: SearchBackfillRun) {
  if (run.queued_batches <= 0) return 100;
  return Math.min(100, Math.round(((run.completed_batches + run.failed_batches) / run.queued_batches) * 100));
}

export default function DebugDigestsPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const { user } = useUser();
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
  const [titleBackfillUserId, setTitleBackfillUserId] = useState("");
  const [titleBackfillLimit, setTitleBackfillLimit] = useState("200");
  const [titleBackfillDryRun, setTitleBackfillDryRun] = useState(true);
  const [busyTitleBackfill, setBusyTitleBackfill] = useState(false);
  const [titleBackfillResult, setTitleBackfillResult] = useState<TitleBackfillResponse | null>(null);
  const [openRouterCostUserId, setOpenRouterCostUserId] = useState("");
  const [openRouterCostLimit, setOpenRouterCostLimit] = useState("200");
  const [openRouterCostFrom, setOpenRouterCostFrom] = useState("");
  const [openRouterCostTo, setOpenRouterCostTo] = useState("");
  const [openRouterCostDryRun, setOpenRouterCostDryRun] = useState(true);
  const [busyOpenRouterCostBackfill, setBusyOpenRouterCostBackfill] = useState(false);
  const [openRouterCostBackfillResult, setOpenRouterCostBackfillResult] = useState<OpenRouterCostBackfillResponse | null>(null);
  const [searchBackfillOffset, setSearchBackfillOffset] = useState("0");
  const [searchBackfillLimit, setSearchBackfillLimit] = useState("500");
  const [searchBackfillAll, setSearchBackfillAll] = useState(false);
  const [busySearchBackfill, setBusySearchBackfill] = useState(false);
  const [busySearchBackfillRuns, setBusySearchBackfillRuns] = useState(false);
  const [searchBackfillResult, setSearchBackfillResult] = useState<SearchBackfillResponse | null>(null);
  const [searchBackfillRuns, setSearchBackfillRuns] = useState<SearchBackfillRun[]>([]);
  const [retrySourceId, setRetrySourceId] = useState("");
  const [busyRetryFailed, setBusyRetryFailed] = useState(false);
  const [retryFailedResult, setRetryFailedResult] = useState<BulkRetryFailedResult | null>(null);
  const [busySystemHealth, setBusySystemHealth] = useState(false);
  const [systemHealth, setSystemHealth] = useState<DebugSystemStatusResponse | null>(null);
  const [webHealth, setWebHealth] = useState<{ status: number; latency_ms: number; body: unknown } | null>(null);
  const [busyOneSignalDebug, setBusyOneSignalDebug] = useState(false);
  const [oneSignalDebug, setOneSignalDebug] = useState<OneSignalDebugState | null>(null);
  const [busyPushTest, setBusyPushTest] = useState(false);
  const [pushTestSubscriptionId, setPushTestSubscriptionId] = useState("");
  const [pushTestTitle, setPushTestTitle] = useState("Sifto: テスト通知");
  const [pushTestMessage, setPushTestMessage] = useState("デバッグ画面からのテスト通知です。");
  const [pushTestResult, setPushTestResult] = useState<PushTestResponse | null>(null);
  const [activeSection, setActiveSection] = useState<DebugSection>("system");

  const helperText = useMemo(
    () =>
      t("debug.digest.helperText"),
    [t]
  );
  const loadSearchBackfillRuns = useCallback(async () => {
    setBusySearchBackfillRuns(true);
    try {
      const res = await fetch("/api/debug/search/backfill?limit=12", { cache: "no-store" });
      const text = await res.text();
      if (!res.ok) {
        throw new Error(`${res.status}: ${text || res.statusText}`);
      }
      const data = text ? (JSON.parse(text) as SearchBackfillRunsResponse) : { runs: [] };
      setSearchBackfillRuns(data.runs ?? []);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setError(message);
    } finally {
      setBusySearchBackfillRuns(false);
    }
  }, []);

  useEffect(() => {
    void loadSearchBackfillRuns();
  }, [loadSearchBackfillRuns]);

  useEffect(() => {
    if (!searchBackfillRuns.some((run) => run.status === "queued" || run.status === "running")) {
      return;
    }
    const timer = window.setInterval(() => {
      void loadSearchBackfillRuns();
    }, 5000);
    return () => window.clearInterval(timer);
  }, [loadSearchBackfillRuns, searchBackfillRuns]);

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
  const cacheWindowRowsUser = useMemo(() => {
    const rows = systemHealth?.data?.cache_stats_by_window_user ?? {};
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

  const loadOneSignalDebug = async () => {
    setBusyOneSignalDebug(true);
    try {
      const appId = process.env.NEXT_PUBLIC_ONESIGNAL_APP_ID?.trim() ?? "";
      const permission = typeof Notification !== "undefined" ? Notification.permission : "unsupported";
      const registrations = "serviceWorker" in navigator ? await navigator.serviceWorker.getRegistrations() : [];
      const scopes = registrations.map((r) => r.scope);
      const swRegistrations = registrations.map((r) => ({
        scope: r.scope,
        script_url: r.active?.scriptURL ?? r.waiting?.scriptURL ?? r.installing?.scriptURL ?? null,
      }));
      const rawOs = typeof window !== "undefined" ? window.OneSignal : undefined;
      const os =
        rawOs && !Array.isArray(rawOs) && typeof (rawOs as { init?: unknown }).init === "function"
          ? rawOs
          : undefined;
      const deferredQueue = typeof window !== "undefined" ? window.OneSignalDeferred : undefined;
      const sub = os?.User?.PushSubscription as unknown as Record<string, unknown> | undefined;
      const subscriptionIdRaw = sub?.["id"];
      const subscriptionId = typeof subscriptionIdRaw === "string" && subscriptionIdRaw.length > 0 ? subscriptionIdRaw : null;
      const optedIn = typeof sub?.["optedIn"] === "boolean" ? (sub?.["optedIn"] as boolean) : null;

      let workerFileReachable: boolean | null = null;
      let workerUpdaterReachable: boolean | null = null;
      try {
        const [w1, w2] = await Promise.all([
          fetch("/onesignal/OneSignalSDKWorker.js", { cache: "no-store" }),
          fetch("/onesignal/OneSignalSDKUpdaterWorker.js", { cache: "no-store" }),
        ]);
        workerFileReachable = w1.ok;
        workerUpdaterReachable = w2.ok;
      } catch {
        workerFileReachable = false;
        workerUpdaterReachable = false;
      }

      setOneSignalDebug({
        checked_at: new Date().toISOString(),
        app_id_configured: appId.length > 0,
        app_id_preview: appId ? `${appId.slice(0, 8)}...` : null,
        script_loading: Boolean(window.__siftoOneSignalLoading),
        script_loaded: Boolean(window.__siftoOneSignalScriptLoaded),
        script_error: window.__siftoOneSignalScriptError ?? null,
        deferred_executed: Boolean(window.__siftoOneSignalDeferredExecuted),
        init_enqueued_count: window.__siftoOneSignalInitEnqueued ?? 0,
        sdk_ready: Boolean(window.__siftoOneSignalReady),
        sdk_loaded: Boolean(os),
        deferred_queue_length: Array.isArray(deferredQueue) ? deferredQueue.length : 0,
        init_error: window.__siftoOneSignalInitError ?? null,
        permission,
        opted_in: optedIn,
        login_external_id: user?.primaryEmailAddress?.emailAddress ?? null,
        subscription_id: subscriptionId,
        sw_scopes: scopes,
        sw_registrations: swRegistrations,
        worker_file_reachable: workerFileReachable,
        worker_updater_reachable: workerUpdaterReachable,
      });
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setBusyOneSignalDebug(false);
    }
  };

  useEffect(() => {
    loadOneSignalDebug();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.primaryEmailAddress?.emailAddress]);

  useEffect(() => {
    if (!oneSignalDebug?.subscription_id) return;
    setPushTestSubscriptionId(oneSignalDebug.subscription_id);
  }, [oneSignalDebug?.subscription_id]);

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

  const onPushTest = async (e: FormEvent) => {
    e.preventDefault();
    setBusyPushTest(true);
    setError(null);
    setPushTestResult(null);
    try {
      const res = await postJSON<PushTestResponse>("/api/debug/push/test", {
        subscription_id: pushTestSubscriptionId.trim() || undefined,
        title: pushTestTitle.trim(),
        message: pushTestMessage.trim(),
      });
      setPushTestResult(res);
      showToast("Push test sent", "success");
      await loadOneSignalDebug();
    } catch (e) {
      setError(String(e));
      showToast(String(e), "error");
    } finally {
      setBusyPushTest(false);
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
      showToast(t("debug.retryPending.queued"), "success");
    } catch (e) {
      setError(String(e));
      showToast(String(e), "error");
    } finally {
      setBusyRetryFailed(false);
    }
  };

  const onBackfillTranslatedTitles = async (e: FormEvent) => {
    e.preventDefault();
    setBusyTitleBackfill(true);
    setError(null);
    setTitleBackfillResult(null);
    try {
      const parsedLimit = Number(titleBackfillLimit);
      if (!Number.isFinite(parsedLimit) || parsedLimit < 1 || parsedLimit > 2000) {
        throw new Error("limit must be between 1 and 2000");
      }
      const payload: { user_id?: string; limit: number; dry_run: boolean } = {
        limit: parsedLimit,
        dry_run: titleBackfillDryRun,
      };
      if (titleBackfillUserId.trim()) payload.user_id = titleBackfillUserId.trim();
      const res = await postJSON<TitleBackfillResponse>("/api/debug/titles/backfill", payload);
      setTitleBackfillResult(res);
      showToast(
        titleBackfillDryRun ? "Title backfill dry-run completed" : "Title backfill completed",
        "success"
      );
    } catch (e) {
      setError(String(e));
      showToast(String(e), "error");
    } finally {
      setBusyTitleBackfill(false);
    }
  };

  const onBackfillOpenRouterCosts = async (e: FormEvent) => {
    e.preventDefault();
    setBusyOpenRouterCostBackfill(true);
    setError(null);
    setOpenRouterCostBackfillResult(null);
    try {
      const parsedLimit = Number(openRouterCostLimit);
      if (!Number.isFinite(parsedLimit) || parsedLimit <= 0 || parsedLimit > 5000) {
        throw new Error(t("debug.openrouterCost.limitError"));
      }
      const payload: Record<string, unknown> = {
        limit: parsedLimit,
        dry_run: openRouterCostDryRun,
      };
      if (openRouterCostUserId.trim()) payload.user_id = openRouterCostUserId.trim();
      if (openRouterCostFrom.trim()) payload.from = openRouterCostFrom.trim();
      if (openRouterCostTo.trim()) payload.to = openRouterCostTo.trim();
      const res = await postJSON<OpenRouterCostBackfillResponse>("/api/debug/llm-usage/backfill-openrouter-costs", payload);
      setOpenRouterCostBackfillResult(res);
      showToast(
        openRouterCostDryRun ? t("debug.openrouterCost.previewDone") : t("debug.openrouterCost.runDone"),
        "success"
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setError(message);
      showToast(message, "error");
    } finally {
      setBusyOpenRouterCostBackfill(false);
    }
  };

  const onBackfillSearch = async (e: FormEvent) => {
    e.preventDefault();
    setBusySearchBackfill(true);
    setError(null);
    setSearchBackfillResult(null);
    try {
      const parsedOffset = Number(searchBackfillOffset);
      const parsedLimit = Number(searchBackfillLimit);
      if (!Number.isFinite(parsedOffset) || parsedOffset < 0) {
        throw new Error(t("debug.searchBackfill.offsetError"));
      }
      if (!Number.isFinite(parsedLimit) || parsedLimit < 1 || parsedLimit > 5000) {
        throw new Error(t("debug.searchBackfill.limitError"));
      }
      const res = await postJSON<SearchBackfillResponse>("/api/debug/search/backfill", {
        offset: parsedOffset,
        limit: parsedLimit,
        all: searchBackfillAll,
      });
      setSearchBackfillResult(res);
      showToast(t("debug.searchBackfill.runDone"), "success");
      await loadSearchBackfillRuns();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setError(message);
      showToast(message, "error");
    } finally {
      setBusySearchBackfill(false);
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

      const userMetricID = userId.trim();
      const res = await fetch(
        `/api/debug/system-status${userMetricID ? `?user_id=${encodeURIComponent(userMetricID)}` : ""}`,
        { cache: "no-store" }
      );
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

  const systemSection = (
    <div className="space-y-4">
      <section className="surface-editorial min-w-0 rounded-[28px] p-5">
        <div className="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="min-w-0">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">System</div>
            <h2 className="mt-2 font-serif text-[1.8rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">System Health</h2>
          </div>
          <button
            type="button"
            onClick={loadSystemHealth}
            disabled={busySystemHealth}
            className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-50"
          >
            {busySystemHealth ? t("common.loading") : t("common.refresh")}
          </button>
        </div>
        <div className="grid gap-4 lg:grid-cols-2">
          <div className="min-w-0 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
            <div className="mb-2 text-xs font-medium text-[var(--color-editorial-ink-faint)]">Web /health</div>
            {webHealth ? (
              <div className="space-y-2 text-xs">
                <div className="flex flex-wrap items-center gap-2">
                  <StatusPill ok={webHealth.status >= 200 && webHealth.status < 400} label={`HTTP ${webHealth.status}`} />
                  <span className="text-[var(--color-editorial-ink-faint)]">{webHealth.latency_ms} ms</span>
                </div>
                <pre className="max-w-full overflow-x-auto rounded-[16px] bg-zinc-950 p-3 text-[11px] text-zinc-100">
                  {JSON.stringify(webHealth.body, null, 2)}
                </pre>
              </div>
            ) : (
              <p className="text-xs text-[var(--color-editorial-ink-faint)]">{t("debug.notFetched")}</p>
            )}
          </div>
          <div className="min-w-0 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
            <div className="mb-2 text-xs font-medium text-[var(--color-editorial-ink-faint)]">API Internal System Status</div>
            {systemHealth ? (
              <div className="space-y-3 text-xs">
                <div className="flex flex-wrap items-center gap-2">
                  <StatusPill ok={systemHealth.data?.status === "ok"} label={systemHealth.data?.status ?? "unknown"} />
                  <span className="text-[var(--color-editorial-ink-faint)]">proxy {systemHealth.proxy_latency_ms} ms</span>
                </div>
                <div className="space-y-2">
                  {Object.entries(systemHealth.data?.checks ?? {}).map(([name, row]) => (
                    <div key={name} className="rounded-[16px] border border-[var(--color-editorial-line)] px-3 py-3">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div className="font-medium text-[var(--color-editorial-ink)]">{name}</div>
                        <div className="flex items-center gap-2">
                          <StatusPill ok={row.status === "ok"} label={row.status ?? "unknown"} />
                          {typeof row.latency_ms === "number" && (
                            <span className="text-[var(--color-editorial-ink-faint)]">{row.latency_ms} ms</span>
                          )}
                          {typeof row.http_status === "number" && row.http_status > 0 && (
                            <span className="text-[var(--color-editorial-ink-faint)]">HTTP {row.http_status}</span>
                          )}
                        </div>
                      </div>
                      {row.detail && <div className="mt-1 text-[var(--color-editorial-ink-faint)]">{row.detail}</div>}
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <p className="text-xs text-[var(--color-editorial-ink-faint)]">{t("debug.notFetched")}</p>
            )}
          </div>
        </div>
      </section>

      <div className="grid gap-4 lg:grid-cols-2">
        <section className="surface-editorial min-w-0 rounded-[28px] p-5">
          <div className="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <h2 className="text-sm font-semibold text-[var(--color-editorial-ink)]">OneSignal Debug</h2>
            <button
              type="button"
              onClick={loadOneSignalDebug}
              disabled={busyOneSignalDebug}
              className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-50"
          >
            {busyOneSignalDebug ? t("common.loading") : t("common.refresh")}
          </button>
          </div>
          {oneSignalDebug ? (
            <pre className="max-w-full overflow-x-auto rounded-[18px] bg-zinc-950 p-3 text-xs text-zinc-100">
              {JSON.stringify(oneSignalDebug, null, 2)}
            </pre>
          ) : (
            <p className="text-xs text-[var(--color-editorial-ink-faint)]">{t("debug.notFetched")}</p>
          )}
        </section>

        <section className="surface-editorial min-w-0 rounded-[28px] p-5">
          <h2 className="mb-3 text-sm font-semibold text-[var(--color-editorial-ink)]">Push Test</h2>
          <form onSubmit={onPushTest} className="space-y-3">
            <label className="block text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">Subscription ID (optional)</div>
              <input
                value={pushTestSubscriptionId}
                onChange={(e) => setPushTestSubscriptionId(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
                placeholder="onesignal subscription id"
              />
            </label>
            <label className="block text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">Title</div>
              <input
                value={pushTestTitle}
                onChange={(e) => setPushTestTitle(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
                placeholder="notification title"
              />
            </label>
            <label className="block text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">Message</div>
              <input
                value={pushTestMessage}
                onChange={(e) => setPushTestMessage(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
                placeholder="notification body"
              />
            </label>
            <button
              type="submit"
              disabled={busyPushTest}
              className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-xs font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-95 disabled:opacity-60"
            >
              {busyPushTest ? t("debug.running") : "Send Push Test"}
            </button>
          </form>
          {pushTestResult && (
            <pre className="mt-3 max-w-full overflow-x-auto rounded-[18px] bg-zinc-950 p-3 text-xs text-zinc-100">
              {JSON.stringify(pushTestResult, null, 2)}
            </pre>
          )}
        </section>
      </div>

      <section className="surface-editorial min-w-0 rounded-[28px] p-5">
        <h2 className="mb-3 text-sm font-semibold text-[var(--color-editorial-ink)]">Cache Hit Rate</h2>
        {!systemHealth ? (
          <p className="text-xs text-[var(--color-editorial-ink-faint)]">{t("debug.systemHealthFirst")}</p>
        ) : (
          <div className="space-y-4">
            <div className="min-w-0 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
              <div className="mb-2 text-xs font-medium text-[var(--color-editorial-ink-faint)]">Current Process Counters</div>
              <div className="grid gap-2 sm:grid-cols-2">
                {Object.entries(systemHealth.data?.cache_stats ?? {}).map(([name, stat]) => (
                  <div key={name} className="min-w-0 rounded-[16px] border border-[var(--color-editorial-line)] px-3 py-3">
                    <div className="mb-1 break-all text-[11px] font-medium text-[var(--color-editorial-ink)]">{name}</div>
                    <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-[11px] text-[var(--color-editorial-ink-soft)]">
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
              <div className="min-w-0 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="mb-2 text-xs font-medium text-[var(--color-editorial-ink-faint)]">Windowed Hit Rate</div>
                <div className="max-w-full overflow-x-auto">
                  <table className="min-w-[540px] border-separate border-spacing-0 text-[11px]">
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

            {cacheWindowRowsUser.length > 0 && (
              <div className="min-w-0 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="mb-2 break-all text-xs font-medium text-[var(--color-editorial-ink-faint)]">
                  User Windowed Hit Rate ({systemHealth.data?.cache_metrics_user_id ?? "n/a"})
                </div>
                <div className="max-w-full overflow-x-auto">
                  <table className="min-w-[540px] border-separate border-spacing-0 text-[11px]">
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
                      {cacheWindowRowsUser.map((row) => (
                        <tr key={`user-${row.window}`}>
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
                  <div key={win} className="rounded-[16px] border border-[var(--color-editorial-line)] px-3 py-3">
                    <div className="mb-1 text-[11px] font-medium text-[var(--color-editorial-ink)]">{win}</div>
                    {"error" in row && row.error ? (
                      <div className="text-[11px] text-red-600">{row.error}</div>
                    ) : (
                      <div className="grid gap-2 sm:grid-cols-2">
                        {(["dashboard", "reading_plan"] as const).map((k) => {
                          const v = row[k];
                          const rate = typeof v?.hit_rate === "number" ? `${(v.hit_rate * 100).toFixed(1)}%` : "N/A";
                          return (
                            <div key={k} className="rounded-[14px] bg-[var(--color-editorial-panel-strong)] px-3 py-2">
                              <div className="flex items-center justify-between text-[11px]">
                                <span className="font-medium text-[var(--color-editorial-ink-soft)]">{k}</span>
                                <span className="text-[var(--color-editorial-ink)]">{rate}</span>
                              </div>
                              <div className="mt-0.5 text-[10px] text-[var(--color-editorial-ink-faint)]">
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
    </div>
  );

  const digestOpsSection = (
    <div className="space-y-4">
      <section className="surface-editorial rounded-[28px] p-5">
        <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">Digest Ops</div>
        <h2 className="mt-2 font-serif text-[1.8rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">Generate / Send / Inspect</h2>
        <p className="mt-3 max-w-3xl text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{helperText}</p>
      </section>

      <section className="surface-editorial rounded-[28px] p-5">
        <h2 className="mb-3 text-sm font-semibold text-[var(--color-editorial-ink)]">Generate Digest</h2>
        <form onSubmit={onGenerate} className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">User ID (optional)</div>
              <input
                value={userId}
                onChange={(e) => setUserId(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
                placeholder="all users if empty"
              />
            </label>
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">Digest Date (JST)</div>
              <input
                type="date"
                value={digestDate}
                onChange={(e) => setDigestDate(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
              />
            </label>
          </div>
          <label className="flex items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)]">
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
            className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-95 disabled:opacity-50"
          >
            {busyGenerate ? t("debug.running") : "Generate Debug"}
          </button>
        </form>

        {generateResult && (
          <pre className="mt-4 overflow-x-auto rounded-[18px] bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(generateResult, null, 2)}
          </pre>
        )}
      </section>

      <section className="surface-editorial rounded-[28px] p-5">
        <h2 className="mb-3 text-sm font-semibold text-[var(--color-editorial-ink)]">Send Digest</h2>
        <form onSubmit={onSend} className="flex flex-col gap-3 sm:flex-row sm:items-end">
          <label className="flex-1 text-sm">
            <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">Digest ID</div>
            <input
              value={digestId}
              onChange={(e) => setDigestId(e.target.value)}
              className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
              placeholder="00000000-0000-0000-0000-0000000000dd"
            />
          </label>
          <button
            type="submit"
            disabled={busySend || !digestId.trim()}
            className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-95 disabled:opacity-50"
          >
            {busySend ? t("debug.running") : "Queue Send"}
          </button>
          <button
            type="button"
            disabled={busyInspect || !digestId.trim()}
            onClick={inspectDigest}
            className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-50"
          >
            {busyInspect ? t("debug.inspecting") : t("debug.statusCheck")}
          </button>
        </form>

        {sendResult && (
          <pre className="mt-4 overflow-x-auto rounded-[18px] bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(sendResult, null, 2)}
          </pre>
        )}

        {digestDetail && (
          <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4 text-sm">
            <div><span className="font-medium">digest_id:</span> {digestDetail.id}</div>
            <div><span className="font-medium">send_status:</span> {digestDetail.send_status ?? "-"}</div>
            <div><span className="font-medium">send_tried_at:</span> {digestDetail.send_tried_at ?? "-"}</div>
            <div><span className="font-medium">sent_at:</span> {digestDetail.sent_at ?? "-"}</div>
            <div><span className="font-medium">email_copy:</span> {digestDetail.email_subject && digestDetail.email_body ? "generated" : "not generated"}</div>
            {digestDetail.send_error && (
              <pre className="mt-3 overflow-x-auto rounded-[18px] bg-zinc-950 p-3 text-xs text-zinc-100">
                {digestDetail.send_error}
              </pre>
            )}
          </div>
        )}
      </section>
    </div>
  );

  const backfillsSection = (
    <div className="space-y-4">
      <section className="surface-editorial rounded-[28px] p-5">
        <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">Backfills</div>
        <h2 className="mt-2 font-serif text-[1.8rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">Embeddings / Titles / OpenRouter Costs</h2>
      </section>

      <section className="surface-editorial rounded-[28px] p-5">
        <h2 className="mb-3 text-sm font-semibold text-[var(--color-editorial-ink)]">Embeddings Backfill</h2>
        <form onSubmit={onBackfillEmbeddings} className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <label className="text-sm sm:col-span-2">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">User ID (optional)</div>
              <input
                value={backfillUserId}
                onChange={(e) => setBackfillUserId(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
                placeholder="all users if empty"
              />
            </label>
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">Limit (1-1000)</div>
              <input
                type="number"
                min={1}
                max={1000}
                value={backfillLimit}
                onChange={(e) => setBackfillLimit(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
              />
            </label>
          </div>
          <label className="flex items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)]">
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
            className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-95 disabled:opacity-50"
          >
            {busyBackfill ? t("debug.running") : backfillDryRun ? "Preview Backfill" : "Queue Backfill"}
          </button>
        </form>

        {backfillResult && (
          <pre className="mt-4 overflow-x-auto rounded-[18px] bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(backfillResult, null, 2)}
          </pre>
        )}
      </section>

      <section className="surface-editorial rounded-[28px] p-5">
        <h2 className="mb-3 text-sm font-semibold text-[var(--color-editorial-ink)]">{t("debug.openrouterCost.title")}</h2>
        <form onSubmit={onBackfillOpenRouterCosts} className="space-y-3">
          <p className="text-xs text-[var(--color-editorial-ink-faint)]">{t("debug.openrouterCost.description")}</p>
          <div className="grid gap-3 sm:grid-cols-3">
            <label className="text-sm sm:col-span-2">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("debug.openrouterCost.userId")}</div>
              <input
                value={openRouterCostUserId}
                onChange={(e) => setOpenRouterCostUserId(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
                placeholder={t("debug.openrouterCost.userPlaceholder")}
              />
            </label>
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("debug.openrouterCost.limit")}</div>
              <input
                type="number"
                min={1}
                max={5000}
                value={openRouterCostLimit}
                onChange={(e) => setOpenRouterCostLimit(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
              />
            </label>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("debug.openrouterCost.from")}</div>
              <input
                value={openRouterCostFrom}
                onChange={(e) => setOpenRouterCostFrom(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
                placeholder={t("debug.openrouterCost.datePlaceholder")}
              />
            </label>
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("debug.openrouterCost.to")}</div>
              <input
                value={openRouterCostTo}
                onChange={(e) => setOpenRouterCostTo(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
                placeholder={t("debug.openrouterCost.datePlaceholder")}
              />
            </label>
          </div>
          <label className="flex items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)]">
            <input
              type="checkbox"
              checked={openRouterCostDryRun}
              onChange={(e) => setOpenRouterCostDryRun(e.target.checked)}
              className="accent-zinc-900"
            />
            {t("debug.digest.dryRun")}
          </label>
          <button
            type="submit"
            disabled={busyOpenRouterCostBackfill}
            className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-95 disabled:opacity-50"
          >
            {busyOpenRouterCostBackfill
              ? t("debug.running")
              : openRouterCostDryRun
                ? t("debug.openrouterCost.preview")
                : t("debug.openrouterCost.run")}
          </button>
        </form>

        {openRouterCostBackfillResult && (
          <pre className="mt-4 overflow-x-auto rounded-[18px] bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(openRouterCostBackfillResult, null, 2)}
          </pre>
        )}
      </section>

      <section className="surface-editorial rounded-[28px] p-5">
        <h2 className="mb-3 text-sm font-semibold text-[var(--color-editorial-ink)]">Translated Title Backfill</h2>
        <form onSubmit={onBackfillTranslatedTitles} className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <label className="text-sm sm:col-span-2">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">User ID (optional)</div>
              <input
                value={titleBackfillUserId}
                onChange={(e) => setTitleBackfillUserId(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
                placeholder="all users if empty"
              />
            </label>
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">Limit (1-2000)</div>
              <input
                type="number"
                min={1}
                max={2000}
                value={titleBackfillLimit}
                onChange={(e) => setTitleBackfillLimit(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
              />
            </label>
          </div>
          <label className="flex items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)]">
            <input
              type="checkbox"
              checked={titleBackfillDryRun}
              onChange={(e) => setTitleBackfillDryRun(e.target.checked)}
              className="accent-zinc-900"
            />
            {t("debug.digest.dryRun")}
          </label>
          <button
            type="submit"
            disabled={busyTitleBackfill}
            className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-95 disabled:opacity-50"
          >
            {busyTitleBackfill ? t("debug.running") : titleBackfillDryRun ? "Preview Backfill" : "Run Backfill"}
          </button>
        </form>

        {titleBackfillResult && (
          <pre className="mt-4 overflow-x-auto rounded-[18px] bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(titleBackfillResult, null, 2)}
          </pre>
        )}
      </section>

      <section className="surface-editorial rounded-[28px] p-5">
        <div className="mb-3 flex items-center justify-between gap-3">
          <h2 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("debug.searchBackfill.title")}</h2>
          <button
            type="button"
            onClick={() => void loadSearchBackfillRuns()}
            disabled={busySearchBackfillRuns}
            className="rounded-full border border-[var(--color-editorial-line)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-50"
          >
            {busySearchBackfillRuns ? t("debug.running") : t("debug.searchBackfill.refresh")}
          </button>
        </div>
        <form onSubmit={onBackfillSearch} className="space-y-3">
          <p className="text-xs text-[var(--color-editorial-ink-faint)]">{t("debug.searchBackfill.description")}</p>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("debug.searchBackfill.offset")}</div>
              <input
                type="number"
                min={0}
                value={searchBackfillOffset}
                onChange={(e) => setSearchBackfillOffset(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
              />
            </label>
            <label className="text-sm">
              <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("debug.searchBackfill.limit")}</div>
              <input
                type="number"
                min={1}
                max={5000}
                value={searchBackfillLimit}
                onChange={(e) => setSearchBackfillLimit(e.target.value)}
                className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
              />
            </label>
          </div>
          <label className="flex items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)]">
            <input
              type="checkbox"
              checked={searchBackfillAll}
              onChange={(e) => setSearchBackfillAll(e.target.checked)}
              className="accent-zinc-900"
            />
            {t("debug.searchBackfill.all")}
          </label>
          <button
            type="submit"
            disabled={busySearchBackfill}
            className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-95 disabled:opacity-50"
          >
            {busySearchBackfill ? t("debug.running") : t("debug.searchBackfill.run")}
          </button>
        </form>

        {searchBackfillResult && (
          <pre className="mt-4 overflow-x-auto rounded-[18px] bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(searchBackfillResult, null, 2)}
          </pre>
        )}

        <div className="mt-4 space-y-3">
          {searchBackfillRuns.length === 0 ? (
            <div className="rounded-[16px] border border-dashed border-[var(--color-editorial-line)] px-4 py-3 text-xs text-[var(--color-editorial-ink-faint)]">
              {t("debug.searchBackfill.noRuns")}
            </div>
          ) : (
            searchBackfillRuns.map((run) => (
              <div key={run.id} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="text-xs font-semibold text-[var(--color-editorial-ink)]">
                    {run.all_items ? t("debug.searchBackfill.modeAll") : t("debug.searchBackfill.modePage")} · {run.status}
                  </div>
                  <div className="text-[11px] text-[var(--color-editorial-ink-faint)]">{new Date(run.created_at).toLocaleString()}</div>
                </div>
                <div className="mt-2 h-2 overflow-hidden rounded-full bg-[var(--color-editorial-line)]">
                  <div className="h-full rounded-full bg-[var(--color-editorial-ink)] transition-all" style={{ width: `${searchBackfillProgress(run)}%` }} />
                </div>
                <div className="mt-3 grid gap-2 text-xs text-[var(--color-editorial-ink-soft)] sm:grid-cols-2 lg:grid-cols-4">
                  <div>{t("debug.searchBackfill.progress")}: {run.completed_batches + run.failed_batches} / {Math.max(run.queued_batches, 0)}</div>
                  <div>{t("debug.searchBackfill.processed")}: {run.processed_items} / {run.total_items}</div>
                  <div>{t("debug.searchBackfill.offset")}: {run.requested_offset}</div>
                  <div>{t("debug.searchBackfill.limit")}: {run.batch_size}</div>
                </div>
                {run.last_error ? (
                  <div className="mt-3 rounded-[14px] border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900">
                    {run.last_error}
                  </div>
                ) : null}
              </div>
            ))
          )}
        </div>
      </section>
    </div>
  );

  const recoverySection = (
    <div className="space-y-4">
      <section className="surface-editorial rounded-[28px] p-5">
        <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">Recovery</div>
        <h2 className="mt-2 font-serif text-[1.8rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">Retry Failed Items</h2>
      </section>

      <section className="surface-editorial rounded-[28px] p-5">
        <h2 className="mb-3 text-sm font-semibold text-[var(--color-editorial-ink)]">{t("debug.retryPending.title")}</h2>
        <form onSubmit={onRetryFailedItems} className="space-y-4">
          <label className="block text-sm">
            <div className="mb-1 text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("debug.retryPending.sourceId")}</div>
            <input
              value={retrySourceId}
              onChange={(e) => setRetrySourceId(e.target.value)}
              className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm outline-none"
              placeholder={t("debug.retryPending.sourcePlaceholder")}
            />
          </label>
          <p className="text-xs text-[var(--color-editorial-ink-faint)]">{t("debug.retryPending.description")}</p>
          <div className="pt-1">
            <button
              type="submit"
              disabled={busyRetryFailed}
              className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-95 disabled:opacity-50"
            >
              {busyRetryFailed ? t("debug.running") : t("debug.retryPending.run")}
            </button>
          </div>
        </form>

        {retryFailedResult && (
          <pre className="mt-4 overflow-x-auto rounded-[18px] bg-zinc-950 p-3 text-xs text-zinc-100">
            {JSON.stringify(retryFailedResult, null, 2)}
          </pre>
        )}
      </section>
    </div>
  );

  return (
    <PageTransition>
      <div className="space-y-6">
      <PageHeader
        title={t("nav.debug")}
        titleIcon={Bug}
        description={helperText}
      />

        {error && (
          <div className="rounded-[22px] border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 shadow-[var(--shadow-card)]">
            {error}
          </div>
        )}

        <div className="grid gap-5 xl:grid-cols-[260px_minmax(0,1fr)]">
          <aside className="surface-editorial rounded-[24px] p-4">
            {[
              { key: "system" as const, label: "System", meta: "health, OneSignal, push test, cache" },
              { key: "digestOps" as const, label: "Digest Ops", meta: "generate, send, inspect" },
              { key: "backfills" as const, label: "Backfills", meta: "embeddings, titles, search, OpenRouter costs" },
              { key: "recovery" as const, label: "Recovery", meta: "retry failed items" },
            ].map((section) => (
              <button
                key={section.key}
                type="button"
                onClick={() => setActiveSection(section.key)}
                className={`relative block w-full rounded-[16px] px-4 py-[13px] text-left ${
                  activeSection === section.key
                    ? "bg-[linear-gradient(90deg,rgba(243,236,227,0.92),rgba(243,236,227,0.28)_78%,transparent)]"
                    : "bg-transparent"
                }`}
              >
                {activeSection === section.key ? (
                  <span className="absolute bottom-3 left-0 top-3 w-[3px] rounded-full bg-[var(--color-editorial-ink)]" />
                ) : null}
                <div className="text-[15px] font-semibold text-[var(--color-editorial-ink)]">{section.label}</div>
                <div className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-faint)]">{section.meta}</div>
              </button>
            ))}
          </aside>

          <section className="min-w-0">
            {activeSection === "system" ? systemSection : null}
            {activeSection === "digestOps" ? digestOpsSection : null}
            {activeSection === "backfills" ? backfillsSection : null}
            {activeSection === "recovery" ? recoverySection : null}
          </section>
        </div>
      </div>
    </PageTransition>
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
