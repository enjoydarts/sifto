"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { api, Digest } from "@/lib/api";

export default function DigestsPage() {
  const [digests, setDigests] = useState<Digest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getDigests()
      .then((d) => setDigests(d ?? []))
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div>
      <h1 className="mb-6 text-2xl font-bold">Digests</h1>

      {loading && <p className="text-sm text-zinc-500">Loading…</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}
      {!loading && digests.length === 0 && (
        <p className="text-sm text-zinc-400">No digests yet.</p>
      )}

      <ul className="space-y-2">
        {digests.map((d) => (
          <li key={d.id}>
            <Link
              href={`/digests/${d.id}`}
              className="flex items-center justify-between rounded-lg border border-zinc-200 bg-white px-4 py-3 transition-colors hover:bg-zinc-50"
            >
              <div>
                <div className="font-medium text-zinc-900">{d.digest_date}</div>
                <div className="text-xs text-zinc-400">
                  {new Date(d.created_at).toLocaleString("ja-JP")}
                </div>
              </div>
              {d.sent_at ? (
                <span className="rounded bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700">
                  Sent
                </span>
              ) : (
                <span className="rounded bg-zinc-100 px-2 py-0.5 text-xs font-medium text-zinc-500">
                  Pending
                </span>
              )}
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
