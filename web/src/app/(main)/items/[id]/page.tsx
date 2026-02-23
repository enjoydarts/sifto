"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, ItemDetail } from "@/lib/api";

const STATUS_COLOR: Record<string, string> = {
  new: "bg-zinc-100 text-zinc-600",
  fetched: "bg-blue-50 text-blue-600",
  facts_extracted: "bg-purple-50 text-purple-600",
  summarized: "bg-green-50 text-green-700",
  failed: "bg-red-50 text-red-600",
};

export default function ItemDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [item, setItem] = useState<ItemDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getItem(id)
      .then(setItem)
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <p className="text-sm text-zinc-500">Loading…</p>;
  if (error) return <p className="text-sm text-red-500">{error}</p>;
  if (!item) return null;

  return (
    <div>
      <Link
        href="/items"
        className="mb-4 inline-block text-sm text-zinc-500 hover:text-zinc-900"
      >
        ← Items
      </Link>

      <div className="mb-3 flex items-center gap-2">
        <span
          className={`rounded px-2 py-0.5 text-xs font-medium ${
            STATUS_COLOR[item.status] ?? "bg-zinc-100 text-zinc-600"
          }`}
        >
          {item.status}
        </span>
        {item.published_at && (
          <span className="text-sm text-zinc-500">
            {new Date(item.published_at).toLocaleDateString("ja-JP")}
          </span>
        )}
      </div>

      <h1 className="mb-2 text-2xl font-bold leading-snug text-zinc-900">
        {item.title ?? "No title"}
      </h1>
      <a
        href={item.url}
        target="_blank"
        rel="noopener noreferrer"
        className="mb-6 block truncate text-sm text-blue-600 hover:underline"
      >
        {item.url}
      </a>

      {/* Summary */}
      {item.summary && (
        <section className="mb-6 rounded-lg border border-green-100 bg-green-50 p-4">
          <h2 className="mb-2 text-sm font-semibold text-green-800">Summary</h2>
          <p className="text-sm leading-relaxed text-green-900">
            {item.summary.summary}
          </p>
          {item.summary.topics.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-1">
              {item.summary.topics.map((t) => (
                <span
                  key={t}
                  className="rounded bg-green-100 px-2 py-0.5 text-xs text-green-700"
                >
                  {t}
                </span>
              ))}
            </div>
          )}
          {item.summary.score != null && (
            <div className="mt-2 text-xs text-green-700">
              Score: {item.summary.score.toFixed(2)}
            </div>
          )}
        </section>
      )}

      {/* Facts */}
      {item.facts && item.facts.facts.length > 0 && (
        <section className="mb-6">
          <h2 className="mb-2 text-sm font-semibold text-zinc-700">Facts</h2>
          <ul className="space-y-1">
            {item.facts.facts.map((f, i) => (
              <li key={i} className="flex gap-2 text-sm text-zinc-700">
                <span className="shrink-0 text-zinc-400">{i + 1}.</span>
                <span>{f}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {/* Content */}
      {item.content_text && (
        <section>
          <h2 className="mb-2 text-sm font-semibold text-zinc-700">Content</h2>
          <div className="max-h-96 overflow-y-auto rounded-lg border border-zinc-200 bg-white p-4 text-sm leading-relaxed whitespace-pre-wrap text-zinc-700">
            {item.content_text}
          </div>
        </section>
      )}
    </div>
  );
}
