"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { api, Digest } from "@/lib/api";
import Pagination from "@/components/pagination";
import { useI18n } from "@/components/i18n-provider";

export default function DigestsPage() {
  const { t, locale } = useI18n();
  const [digests, setDigests] = useState<Digest[]>([]);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const pageSize = 10;

  useEffect(() => {
    api
      .getDigests()
      .then((d) => setDigests(d ?? []))
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  }, []);

  const paged = digests.slice((page - 1) * pageSize, page * pageSize);

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold">{t("digests.title")}</h1>
        <p className="mt-1 text-sm text-zinc-500">
          {digests.length.toLocaleString()} {t("common.rows")}
        </p>
      </div>

      {loading && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}
      {!loading && digests.length === 0 && (
        <p className="text-sm text-zinc-400">{t("digests.empty")}</p>
      )}

      <ul className="space-y-2">
        {paged.map((d) => (
          <li key={d.id}>
            <Link
              href={`/digests/${d.id}`}
              className="flex items-center justify-between rounded-xl border border-zinc-200 bg-white px-4 py-3 shadow-sm transition-colors hover:bg-zinc-50"
            >
              <div>
                <div className="font-medium text-zinc-900">{d.digest_date}</div>
                <div className="text-xs text-zinc-400">
                  {new Date(d.created_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US")}
                </div>
              </div>
              {d.sent_at ? (
                <span className="rounded bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700">
                  {t("digests.sent")}
                </span>
              ) : (
                <span className="rounded bg-zinc-100 px-2 py-0.5 text-xs font-medium text-zinc-500">
                  {t("digests.pending")}
                </span>
              )}
            </Link>
          </li>
        ))}
      </ul>
      <Pagination total={digests.length} page={page} pageSize={pageSize} onPageChange={setPage} />
    </div>
  );
}
