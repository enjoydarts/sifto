"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, DigestDetail } from "@/lib/api";

export default function DigestDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [digest, setDigest] = useState<DigestDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getDigest(id)
      .then(setDigest)
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <p className="text-sm text-zinc-500">Loading…</p>;
  if (error) return <p className="text-sm text-red-500">{error}</p>;
  if (!digest) return null;

  return (
    <div>
      <Link
        href="/digests"
        className="mb-4 inline-block text-sm text-zinc-500 hover:text-zinc-900"
      >
        ← Digests
      </Link>

      <div className="mb-6 flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-bold">Digest — {digest.digest_date}</h1>
        {digest.sent_at ? (
          <span className="rounded bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700">
            Sent {new Date(digest.sent_at).toLocaleString("ja-JP")}
          </span>
        ) : (
          <span className="rounded bg-zinc-100 px-2 py-0.5 text-xs font-medium text-zinc-500">
            Not sent
          </span>
        )}
      </div>

      {digest.items.length === 0 ? (
        <p className="text-sm text-zinc-400">No items in this digest.</p>
      ) : (
        <div className="space-y-4">
          {digest.items.map((di) => (
            <div
              key={di.item.id}
              className="rounded-lg border border-zinc-200 bg-white p-4"
            >
              <div className="mb-3 flex items-start gap-3">
                <span className="shrink-0 text-sm font-bold text-zinc-400">
                  #{di.rank}
                </span>
                <div className="min-w-0 flex-1">
                  <Link
                    href={`/items/${di.item.id}`}
                    className="block text-sm font-semibold text-zinc-900 hover:underline"
                  >
                    {di.item.title ?? di.item.url}
                  </Link>
                  <a
                    href={di.item.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="block truncate text-xs text-zinc-400 hover:text-blue-500"
                  >
                    {di.item.url}
                  </a>
                </div>
              </div>

              <p className="mb-3 pl-6 text-sm leading-relaxed text-zinc-700">
                {di.summary.summary}
              </p>

              {di.summary.topics.length > 0 && (
                <div className="flex flex-wrap gap-1 pl-6">
                  {di.summary.topics.map((t) => (
                    <span
                      key={t}
                      className="rounded bg-zinc-100 px-2 py-0.5 text-xs text-zinc-600"
                    >
                      {t}
                    </span>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
