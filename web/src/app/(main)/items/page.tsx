"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { api, Item } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import Pagination from "@/components/pagination";
import { useToast } from "@/components/toast-provider";

const STATUS_COLOR: Record<string, string> = {
  new: "bg-zinc-100 text-zinc-600",
  fetched: "bg-blue-50 text-blue-600",
  facts_extracted: "bg-purple-50 text-purple-600",
  summarized: "bg-green-50 text-green-700",
  failed: "bg-red-50 text-red-600",
};

const FILTERS = ["", "summarized", "new", "fetched", "facts_extracted", "failed"] as const;
type SortMode = "newest" | "score";

function scoreTone(score: number) {
  if (score >= 0.8) return "bg-green-50 text-green-700 border-green-200";
  if (score >= 0.65) return "bg-blue-50 text-blue-700 border-blue-200";
  if (score >= 0.5) return "bg-zinc-50 text-zinc-700 border-zinc-200";
  return "bg-amber-50 text-amber-700 border-amber-200";
}

export default function ItemsPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const [items, setItems] = useState<Item[]>([]);
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(1);
  const [sortMode, setSortMode] = useState<SortMode>("newest");
  const pageSize = 20;
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryingIds, setRetryingIds] = useState<Record<string, boolean>>({});

  const load = useCallback(async (status: string) => {
    setLoading(true);
    try {
      const data = await api.getItems(status ? { status } : undefined);
      setItems(data ?? []);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    setPage(1);
    load(filter);
  }, [filter, load]);

  useEffect(() => {
    setPage(1);
  }, [sortMode]);

  const retryItem = useCallback(
    async (itemId: string) => {
      setRetryingIds((prev) => ({ ...prev, [itemId]: true }));
      try {
        await api.retryItem(itemId);
        showToast(locale === "ja" ? "再試行をキュー投入しました" : "Retry queued", "success");
        await load(filter);
      } catch (e) {
        setError(String(e));
        showToast(`${t("common.error")}: ${String(e)}`, "error");
      } finally {
        setRetryingIds((prev) => {
          const next = { ...prev };
          delete next[itemId];
          return next;
        });
      }
    },
    [filter, load, locale, showToast, t]
  );

  const sortedItems = [...items].sort((a, b) => {
    if (sortMode === "score") {
      const as = a.summary_score ?? -1;
      const bs = b.summary_score ?? -1;
      if (bs !== as) return bs - as;
    }
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  });

  const pagedItems = sortedItems.slice((page - 1) * pageSize, page * pageSize);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">{t("items.title")}</h1>
          <p className="mt-1 text-sm text-zinc-500">
            {items.length.toLocaleString()} {t("common.rows")}
          </p>
        </div>
        <div className="flex items-center gap-1 rounded-lg border border-zinc-200 bg-white p-1">
          <button
            type="button"
            onClick={() => setSortMode("newest")}
            className={`rounded px-3 py-1.5 text-xs font-medium transition-colors ${
              sortMode === "newest"
                ? "bg-zinc-900 text-white"
                : "text-zinc-600 hover:bg-zinc-50"
            }`}
          >
            {locale === "ja" ? "新着順" : "Newest"}
          </button>
          <button
            type="button"
            onClick={() => setSortMode("score")}
            className={`rounded px-3 py-1.5 text-xs font-medium transition-colors ${
              sortMode === "score"
                ? "bg-zinc-900 text-white"
                : "text-zinc-600 hover:bg-zinc-50"
            }`}
          >
            {locale === "ja" ? "スコア順" : "Score"}
          </button>
        </div>
      </div>

      {/* Filter tabs */}
      <div className="mb-4 flex flex-wrap gap-1">
        {FILTERS.map((value) => (
          <button
            key={value}
            onClick={() => setFilter(value)}
            className={`rounded px-3 py-1 text-sm font-medium transition-colors ${
              filter === value
                ? "bg-zinc-900 text-white"
                : "border border-zinc-200 bg-white text-zinc-600 hover:bg-zinc-50"
            }`}
          >
            {t(`items.filter.${value || "all"}`)}
          </button>
        ))}
      </div>

      {/* State */}
      {loading && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}
      {!loading && items.length === 0 && (
        <p className="text-sm text-zinc-400">{t("items.empty")}</p>
      )}

      {/* List */}
      <ul className="space-y-2">
        {pagedItems.map((item) => (
	          <li key={item.id}>
	            <div className="flex items-start gap-3 rounded-xl border border-zinc-200 bg-white px-4 py-3 shadow-sm">
	              <Link
	                href={`/items/${item.id}`}
	                className="flex min-w-0 flex-1 items-start gap-3 transition-colors hover:text-zinc-700"
	              >
	                <span
	                  className={`mt-0.5 shrink-0 rounded px-2 py-0.5 text-xs font-medium ${
	                    STATUS_COLOR[item.status] ?? "bg-zinc-100 text-zinc-600"
	                  }`}
	                >
	                  {t(`status.${item.status}`, item.status)}
	                </span>
	                <div className="min-w-0 flex-1">
	                  <div className="flex items-start gap-2">
	                    <div className="min-w-0 flex-1 truncate text-sm font-medium text-zinc-900">
	                      {item.title ?? item.url}
	                    </div>
	                    {item.summary_score != null ? (
	                      <span
	                        className={`shrink-0 rounded border px-2 py-0.5 text-xs font-semibold ${scoreTone(item.summary_score)}`}
	                        title={locale === "ja" ? "要約スコア" : "Summary score"}
	                      >
	                        {item.summary_score.toFixed(2)}
	                      </span>
	                    ) : (
	                      <span
	                        className="shrink-0 rounded border border-zinc-200 bg-zinc-50 px-2 py-0.5 text-xs font-medium text-zinc-400"
	                        title={locale === "ja" ? "未採点" : "Not scored"}
	                      >
	                        {locale === "ja" ? "未採点" : "N/A"}
	                      </span>
	                    )}
	                  </div>
	                  {item.title && (
	                    <div className="truncate text-xs text-zinc-400">
	                      {item.url}
	                    </div>
	                  )}
	                  <div className="mt-1 flex items-center gap-2">
	                    <div className="h-1.5 w-24 rounded-full bg-zinc-100">
	                      {item.summary_score != null && (
	                        <div
	                          className="h-1.5 rounded-full bg-zinc-800"
	                          style={{ width: `${Math.max(4, item.summary_score * 100)}%` }}
	                        />
	                      )}
	                    </div>
	                    <span className="text-[11px] text-zinc-500">
	                      {item.summary_score != null
	                        ? locale === "ja"
	                          ? "スコア"
	                          : "Score"
	                        : locale === "ja"
	                          ? "未採点"
	                          : "Not scored"}
	                    </span>
	                  </div>
	                  <div className="mt-0.5 text-xs text-zinc-400">
	                    {new Date(
	                      item.published_at ?? item.created_at
	                    ).toLocaleDateString(locale === "ja" ? "ja-JP" : "en-US")}
	                  </div>
	                </div>
	              </Link>
	              {item.status === "failed" && (
	                <button
	                  type="button"
	                  disabled={!!retryingIds[item.id]}
	                  onClick={() => retryItem(item.id)}
	                  className="shrink-0 rounded border border-zinc-300 px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50"
	                >
	                  {retryingIds[item.id] ? t("items.retrying") : t("items.retry")}
	                </button>
	              )}
	            </div>
	          </li>
	        ))}
	      </ul>
      <Pagination total={items.length} page={page} pageSize={pageSize} onPageChange={setPage} />
    </div>
  );
}
