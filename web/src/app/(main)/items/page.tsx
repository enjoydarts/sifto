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

export default function ItemsPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const [items, setItems] = useState<Item[]>([]);
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(1);
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

  const pagedItems = items.slice((page - 1) * pageSize, page * pageSize);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">{t("items.title")}</h1>
          <p className="mt-1 text-sm text-zinc-500">
            {items.length.toLocaleString()} {t("common.rows")}
          </p>
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
	                  <div className="truncate text-sm font-medium text-zinc-900">
	                    {item.title ?? item.url}
	                  </div>
	                  {item.title && (
	                    <div className="truncate text-xs text-zinc-400">
	                      {item.url}
	                    </div>
	                  )}
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
