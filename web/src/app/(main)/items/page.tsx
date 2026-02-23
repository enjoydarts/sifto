"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { api, Item } from "@/lib/api";

const STATUS_LABEL: Record<string, string> = {
  new: "New",
  fetched: "Fetched",
  facts_extracted: "Facts",
  summarized: "Summarized",
  failed: "Failed",
};

const STATUS_COLOR: Record<string, string> = {
  new: "bg-zinc-100 text-zinc-600",
  fetched: "bg-blue-50 text-blue-600",
  facts_extracted: "bg-purple-50 text-purple-600",
  summarized: "bg-green-50 text-green-700",
  failed: "bg-red-50 text-red-600",
};

const FILTERS = [
  { value: "", label: "All" },
  { value: "summarized", label: "Summarized" },
  { value: "new", label: "New" },
  { value: "fetched", label: "Fetched" },
  { value: "facts_extracted", label: "Facts" },
  { value: "failed", label: "Failed" },
];

export default function ItemsPage() {
  const [items, setItems] = useState<Item[]>([]);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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
    load(filter);
  }, [filter, load]);

  return (
    <div>
      <h1 className="mb-6 text-2xl font-bold">Items</h1>

      {/* Filter tabs */}
      <div className="mb-4 flex flex-wrap gap-1">
        {FILTERS.map(({ value, label }) => (
          <button
            key={value}
            onClick={() => setFilter(value)}
            className={`rounded px-3 py-1 text-sm font-medium transition-colors ${
              filter === value
                ? "bg-zinc-900 text-white"
                : "border border-zinc-200 bg-white text-zinc-600 hover:bg-zinc-50"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* State */}
      {loading && <p className="text-sm text-zinc-500">Loading…</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}
      {!loading && items.length === 0 && (
        <p className="text-sm text-zinc-400">No items.</p>
      )}

      {/* List */}
      <ul className="space-y-2">
        {items.map((item) => (
          <li key={item.id}>
            <Link
              href={`/items/${item.id}`}
              className="flex items-start gap-3 rounded-lg border border-zinc-200 bg-white px-4 py-3 transition-colors hover:bg-zinc-50"
            >
              <span
                className={`mt-0.5 shrink-0 rounded px-2 py-0.5 text-xs font-medium ${
                  STATUS_COLOR[item.status] ?? "bg-zinc-100 text-zinc-600"
                }`}
              >
                {STATUS_LABEL[item.status] ?? item.status}
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
                  ).toLocaleDateString("ja-JP")}
                </div>
              </div>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
