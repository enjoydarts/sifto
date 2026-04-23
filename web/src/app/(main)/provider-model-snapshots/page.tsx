"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { RefreshCw, Search } from "lucide-react";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";
import { useI18n } from "@/components/i18n-provider";
import { api, ProviderModelSnapshotEntry, ProviderModelSnapshotListResponse } from "@/lib/api";
import { providerLabel } from "@/lib/model-display";
import { useToast } from "@/components/toast-provider";

const PAGE_SIZE = 100;

function formatDateTime(value: string) {
  return new Date(value).toLocaleString();
}

export default function ProviderModelSnapshotsPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [providerFilter, setProviderFilter] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ProviderModelSnapshotListResponse | null>(null);

  const load = useCallback(async (nextPage: number, nextQuery: string, nextProvider: string) => {
    setLoading(true);
    try {
      const next = await api.getProviderModelSnapshots({
        q: nextQuery.trim() || undefined,
        providers: nextProvider ? [nextProvider] : undefined,
        limit: PAGE_SIZE,
        offset: (nextPage - 1) * PAGE_SIZE,
      });
      setData(next);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(page, query, providerFilter);
  }, [load, page, providerFilter, query]);

  const items = useMemo(() => data?.items ?? [], [data?.items]);
  const providerOptions = useMemo(() => {
    return [...(data?.providers ?? [])].sort((a, b) => a.localeCompare(b));
  }, [data?.providers]);
  const totalPages = useMemo(() => {
    const total = data?.total ?? 0;
    return Math.max(1, Math.ceil(total / PAGE_SIZE));
  }, [data?.total]);

  const handleSync = useCallback(async () => {
    setSyncing(true);
    try {
      const result = await api.syncProviderModelSnapshots();
      await load(page, query, providerFilter);
      showToast(
        t("providerModelSnapshots.syncCompleted")
          .replace("{{providers}}", String(result.providers))
          .replace("{{changes}}", String(result.changes)),
        "success",
      );
    } catch (e) {
      setError(String(e));
      showToast(String(e), "error");
    } finally {
      setSyncing(false);
    }
  }, [load, page, providerFilter, query, showToast, t]);

  return (
    <PageTransition>
      <div className="space-y-6">
        <PageHeader title={t("providerModelSnapshots.title")} description={t("providerModelSnapshots.subtitle")} />

        <section className="overflow-hidden rounded-[28px] border border-[color:rgba(148,163,184,0.28)] bg-[linear-gradient(180deg,rgba(248,250,252,0.94),rgba(255,255,255,0.98))] shadow-[0_24px_70px_rgba(15,23,42,0.08)]">
          <div className="border-b border-[color:rgba(148,163,184,0.2)] px-5 py-4">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
              <select
                value={providerFilter}
                onChange={(e) => {
                  setPage(1);
                  setProviderFilter(e.target.value);
                }}
                className="h-12 rounded-2xl border border-[color:rgba(148,163,184,0.3)] bg-white/80 px-4 text-sm text-slate-700 outline-none lg:w-[220px]"
              >
                <option value="">{t("providerModelSnapshots.filterAll")}</option>
                {providerOptions.map((provider) => (
                  <option key={provider} value={provider}>
                    {providerLabel(provider)}
                  </option>
                ))}
              </select>
              <label className="flex min-w-0 flex-1 items-center gap-3 rounded-2xl border border-[color:rgba(148,163,184,0.3)] bg-white/80 px-4 py-3 text-sm text-slate-600">
                <Search className="size-4 shrink-0" />
                <input
                  value={query}
                  onChange={(e) => {
                    setPage(1);
                    setQuery(e.target.value);
                  }}
                  placeholder={t("providerModelSnapshots.searchPlaceholder")}
                  className="min-w-0 flex-1 bg-transparent outline-none placeholder:text-slate-400"
                />
              </label>
              <button
                type="button"
                onClick={handleSync}
                className="inline-flex h-12 items-center justify-center gap-2 rounded-2xl border border-[color:rgba(148,163,184,0.3)] bg-white/80 px-4 text-sm font-medium text-slate-700 transition hover:bg-white"
              >
                <RefreshCw className={`size-4 ${syncing ? "animate-spin" : ""}`} />
                {syncing ? t("providerModelSnapshots.syncing") : t("providerModelSnapshots.sync")}
              </button>
            </div>
            <p className="mt-3 text-sm text-slate-500">
              {t("providerModelSnapshots.summary")
                .replace("{{count}}", String(items.length))
                .replace("{{total}}", String(data?.total ?? 0))}
            </p>
          </div>

          {error ? (
            <div className="px-5 py-6 text-sm text-red-600">{error}</div>
          ) : loading && !data ? (
            <div className="px-5 py-6 text-sm text-slate-500">{t("common.loading")}</div>
          ) : items.length === 0 ? (
            <div className="px-5 py-6 text-sm text-slate-500">{t("providerModelSnapshots.empty")}</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-[color:rgba(148,163,184,0.16)] text-sm">
                <thead className="bg-[color:rgba(248,250,252,0.9)] text-left text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">
                  <tr>
                    <th className="px-5 py-3">{t("providerModelSnapshots.table.provider")}</th>
                    <th className="px-5 py-3">{t("providerModelSnapshots.table.modelId")}</th>
                    <th className="px-5 py-3">{t("providerModelSnapshots.table.fetchedAt")}</th>
                    <th className="px-5 py-3">{t("providerModelSnapshots.table.status")}</th>
                    <th className="px-5 py-3">{t("providerModelSnapshots.table.error")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[color:rgba(148,163,184,0.14)] bg-white/80">
                  {items.map((item: ProviderModelSnapshotEntry) => (
                    <tr key={`${item.provider}:${item.model_id}`} className="align-top text-slate-700">
                      <td className="whitespace-nowrap px-5 py-4 font-medium">{providerLabel(item.provider)}</td>
                      <td className="px-5 py-4 font-mono text-[13px] text-slate-800">{item.model_id}</td>
                      <td className="whitespace-nowrap px-5 py-4 text-slate-500">{formatDateTime(item.fetched_at)}</td>
                      <td className="px-5 py-4">
                        <span className="inline-flex rounded-full border border-[color:rgba(148,163,184,0.22)] bg-slate-50 px-2.5 py-1 text-xs font-medium text-slate-700">
                          {item.status}
                        </span>
                      </td>
                      <td className="px-5 py-4 text-slate-500">{item.error || "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <div className="flex items-center justify-between border-t border-[color:rgba(148,163,184,0.2)] px-5 py-4 text-sm text-slate-600">
            <span>
              {t("common.page")} {page} / {totalPages}
            </span>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => setPage((current) => Math.max(1, current - 1))}
                disabled={page <= 1}
                className="rounded-xl border border-[color:rgba(148,163,184,0.3)] px-3 py-2 disabled:cursor-not-allowed disabled:opacity-40"
              >
                {t("common.prev")}
              </button>
              <button
                type="button"
                onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
                disabled={page >= totalPages}
                className="rounded-xl border border-[color:rgba(148,163,184,0.3)] px-3 py-2 disabled:cursor-not-allowed disabled:opacity-40"
              >
                {t("common.next")}
              </button>
            </div>
          </div>
        </section>
      </div>
    </PageTransition>
  );
}
