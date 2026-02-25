"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { FileText, Mail, Newspaper, Send, Sparkles, TriangleAlert } from "lucide-react";
import { api, DigestDetail } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

function digestStatusBadge(d: DigestDetail, locale: "ja" | "en") {
  if (d.sent_at) {
    return { label: locale === "ja" ? "送信済み" : "Sent", className: "bg-green-50 text-green-700", withSendIcon: true };
  }
  switch (d.send_status) {
    case "compose_failed":
    case "send_email_failed":
    case "fetch_failed":
    case "user_key_failed":
      return { label: locale === "ja" ? "失敗" : "Failed", className: "bg-red-50 text-red-700", withSendIcon: false };
    case "processing":
      return { label: locale === "ja" ? "処理中" : "Processing", className: "bg-blue-50 text-blue-700", withSendIcon: false };
    case "skipped_resend_disabled":
      return { label: locale === "ja" ? "送信無効" : "Resend off", className: "bg-amber-50 text-amber-700", withSendIcon: false };
    case "skipped_user_disabled":
      return { label: locale === "ja" ? "メール送信OFF" : "Email off", className: "bg-zinc-100 text-zinc-600", withSendIcon: false };
    case "skipped_no_items":
      return { label: locale === "ja" ? "対象なし" : "No items", className: "bg-zinc-100 text-zinc-600", withSendIcon: false };
    default:
      return { label: locale === "ja" ? "未送信" : "Pending", className: "bg-zinc-100 text-zinc-500", withSendIcon: false };
  }
}

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
  const statusBadge = digestStatusBadge(digest, locale);

  return (
    <div className="space-y-6">
      <Link href="/digests" className="inline-block text-sm text-zinc-500 hover:text-zinc-900">
        ← {t("nav.digests")}
      </Link>

      <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
              <Mail className="size-6 text-zinc-500" aria-hidden="true" />
              <span>
                {t("digests.title")} · {digest.digest_date}
              </span>
            </h1>
            <p className="mt-1 text-sm text-zinc-500">
              {locale === "ja" ? "生成されたダイジェスト本文と含まれる記事の確認" : "Review generated digest copy and included items"}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium ${statusBadge.className}`}>
              {statusBadge.withSendIcon && <Send className="size-3.5" aria-hidden="true" />}
              {statusBadge.label}
            </span>
          </div>
        </div>

        <div className="mt-4 grid gap-3 sm:grid-cols-2">
          <div className="rounded-lg border border-zinc-200 bg-zinc-50 p-3">
            <div className="mb-1 text-xs font-medium text-zinc-500">
              {locale === "ja" ? "作成日時" : "Created"}
            </div>
            <div className="text-sm text-zinc-800">
              {new Date(digest.created_at).toLocaleString(dateLocale)}
            </div>
          </div>
          <div className="rounded-lg border border-zinc-200 bg-zinc-50 p-3">
            <div className="mb-1 text-xs font-medium text-zinc-500">
              {locale === "ja" ? "送信日時" : "Sent at"}
            </div>
            <div className="text-sm text-zinc-800">
              {digest.sent_at ? new Date(digest.sent_at).toLocaleString(dateLocale) : "—"}
            </div>
          </div>
        </div>

        {digest.email_subject && (
          <div className="mt-3 rounded-lg border border-zinc-200 bg-zinc-50 p-3">
            <div className="mb-1 flex items-center gap-2 text-xs font-medium text-zinc-500">
              <FileText className="size-3.5" aria-hidden="true" />
              {locale === "ja" ? "メール件名" : "Email Subject"}
            </div>
            <div className="text-sm font-medium leading-6 text-zinc-900">{digest.email_subject}</div>
          </div>
        )}
        {digest.send_error && (
          <div className="mt-3 rounded-lg border border-red-200 bg-red-50 p-3">
            <div className="mb-1 flex items-center gap-2 text-xs font-medium text-red-700">
              <TriangleAlert className="size-3.5" aria-hidden="true" />
              send_error
            </div>
            <pre className="overflow-x-auto whitespace-pre-wrap text-xs leading-5 text-red-800">{digest.send_error}</pre>
          </div>
        )}
      </section>

      {digest.email_body && (
        <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <h2 className="mb-3 inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
            <Sparkles className="size-4 text-zinc-500" aria-hidden="true" />
            {locale === "ja" ? "生成メール本文" : "Generated Email Body"}
          </h2>
          <div className="whitespace-pre-wrap text-[15px] leading-8 text-zinc-800">
            {digest.email_body}
          </div>
        </section>
      )}

      {digest.cluster_drafts && digest.cluster_drafts.length > 0 && (
        <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
              <Sparkles className="size-4 text-zinc-500" aria-hidden="true" />
              {locale === "ja" ? "中間ドラフト（クラスタ別）" : "Intermediate Cluster Drafts"}
            </h2>
            <span className="text-xs text-zinc-400">
              {digest.cluster_drafts.length} {locale === "ja" ? "クラスタ" : "clusters"}
            </span>
          </div>
          <div className="space-y-3">
            {digest.cluster_drafts.map((cd) => (
              <div key={cd.id} className="rounded-lg border border-zinc-200 bg-zinc-50/70 p-4">
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  <span className="rounded-full bg-white px-2 py-0.5 text-xs font-semibold text-zinc-700 ring-1 ring-zinc-200">
                    #{cd.rank}
                  </span>
                  <span className="rounded-full bg-zinc-100 px-2.5 py-0.5 text-xs font-semibold text-zinc-800">
                    {cd.cluster_label}
                  </span>
                  <span className="text-xs text-zinc-500">
                    {locale === "ja" ? `${cd.item_count}件` : `${cd.item_count} items`}
                  </span>
                  {typeof cd.max_score === "number" && (
                    <span className="rounded border border-zinc-200 bg-white px-2 py-0.5 text-xs font-medium text-zinc-600">
                      score {cd.max_score.toFixed(2)}
                    </span>
                  )}
                </div>
                <pre className="whitespace-pre-wrap text-xs leading-6 text-zinc-700">{cd.draft_summary}</pre>
              </div>
            ))}
          </div>
        </section>
      )}

      <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
            <Newspaper className="size-4 text-zinc-500" aria-hidden="true" />
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
                className="rounded-xl border border-zinc-200 bg-zinc-50/70 p-4"
              >
                <div className="mb-3 flex items-start gap-3">
                  <span className="mt-0.5 shrink-0 rounded-full bg-white px-2.5 py-1 text-xs font-semibold text-zinc-600 ring-1 ring-zinc-200">
                    #{di.rank}
                  </span>
                  <div className="min-w-0 flex-1">
                    <Link
                      href={`/items/${di.item.id}`}
                      className="block text-sm font-semibold leading-6 text-zinc-900 hover:underline"
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

                <p className="text-sm leading-7 text-zinc-700">{di.summary.summary}</p>

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
