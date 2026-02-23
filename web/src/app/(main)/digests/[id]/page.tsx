"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, DigestDetail } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

export default function DigestDetailPage() {
  const { t, locale } = useI18n();
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

  const dateLocale = useMemo(() => (locale === "ja" ? "ja-JP" : "en-US"), [locale]);

  if (loading) return <p className="text-sm text-zinc-500">{t("common.loading")}</p>;
  if (error) return <p className="text-sm text-red-500">{error}</p>;
  if (!digest) return null;

  return (
    <div className="space-y-6">
      <Link href="/digests" className="inline-block text-sm text-zinc-500 hover:text-zinc-900">
        ← {t("nav.digests")}
      </Link>

      <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="mb-3 flex flex-wrap items-center gap-3">
          <h1 className="text-2xl font-bold tracking-tight">
            {t("digests.title")} · {digest.digest_date}
          </h1>
          {digest.sent_at ? (
            <span className="rounded bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700">
              {t("digests.sent")} {new Date(digest.sent_at).toLocaleString(dateLocale)}
            </span>
          ) : (
            <span className="rounded bg-zinc-100 px-2 py-0.5 text-xs font-medium text-zinc-500">
              {t("digests.pending")}
            </span>
          )}
          {digest.send_status && (
            <span className="rounded bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
              send_status: {digest.send_status}
            </span>
          )}
        </div>

        {digest.email_subject && (
          <div className="rounded-lg border border-zinc-200 bg-zinc-50 p-3">
            <div className="mb-1 text-xs font-medium text-zinc-500">
              {locale === "ja" ? "メール件名" : "Email Subject"}
            </div>
            <div className="text-sm font-medium text-zinc-900">{digest.email_subject}</div>
          </div>
        )}
        {digest.send_error && (
          <div className="mt-3 rounded-lg border border-red-200 bg-red-50 p-3">
            <div className="mb-1 text-xs font-medium text-red-700">send_error</div>
            <pre className="overflow-x-auto whitespace-pre-wrap text-xs text-red-800">{digest.send_error}</pre>
          </div>
        )}
      </section>

      {digest.email_body && (
        <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <h2 className="mb-3 text-sm font-semibold text-zinc-800">
            {locale === "ja" ? "生成メール本文" : "Generated Email Body"}
          </h2>
          <div className="max-h-[24rem] overflow-y-auto rounded-lg border border-zinc-200 bg-zinc-50 p-4 text-sm leading-relaxed whitespace-pre-wrap text-zinc-700">
            {digest.email_body}
          </div>
        </section>
      )}

      <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-zinc-800">
            {locale === "ja" ? "含まれる記事" : "Items in Digest"}
          </h2>
          <span className="text-xs text-zinc-400">{digest.items.length} {t("common.rows")}</span>
        </div>

        {digest.items.length === 0 ? (
          <p className="text-sm text-zinc-400">{locale === "ja" ? "このダイジェストに記事はありません。" : "No items in this digest."}</p>
        ) : (
          <div className="space-y-3">
            {digest.items.map((di) => (
              <div
                key={di.item.id}
                className="rounded-xl border border-zinc-200 bg-zinc-50 p-4"
              >
                <div className="mb-3 flex items-start gap-3">
                  <span className="mt-0.5 shrink-0 rounded bg-white px-2 py-0.5 text-xs font-semibold text-zinc-500 ring-1 ring-zinc-200">
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
                      className="mt-0.5 block break-all text-xs text-zinc-400 hover:text-blue-500"
                    >
                      {di.item.url}
                    </a>
                  </div>
                </div>

                <p className="text-sm leading-relaxed text-zinc-700">{di.summary.summary}</p>

                {di.summary.topics.length > 0 && (
                  <div className="mt-3 flex flex-wrap gap-1">
                    {di.summary.topics.map((topic) => (
                      <span
                        key={topic}
                        className="rounded bg-white px-2 py-0.5 text-xs text-zinc-600 ring-1 ring-zinc-200"
                      >
                        {topic}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
