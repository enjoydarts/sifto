"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { api, Digest } from "@/lib/api";
import Pagination from "@/components/pagination";
import { useI18n } from "@/components/i18n-provider";

function digestStatusBadge(d: Digest, t: (key: string, fallback?: string) => string) {
  if (d.sent_at) {
    return {
      label: t("digest.status.sent"),
      className: "bg-green-50 text-green-700",
    };
  }
  switch (d.send_status) {
    case "compose_failed":
    case "send_email_failed":
    case "fetch_failed":
    case "user_key_failed":
      return {
        label: t("digest.status.failed"),
        className: "bg-red-50 text-red-700",
      };
    case "processing":
      return {
        label: t("digest.status.processing"),
        className: "bg-blue-50 text-blue-700",
      };
    case "skipped_resend_disabled":
      return {
        label: t("digest.status.resendDisabled"),
        className: "bg-amber-50 text-amber-700",
      };
    case "skipped_user_disabled":
      return {
        label: t("digest.status.emailDisabled"),
        className: "bg-zinc-100 text-zinc-600",
      };
    case "skipped_no_items":
      return {
        label: t("digest.status.noItems"),
        className: "bg-zinc-100 text-zinc-600",
      };
    default:
      return {
        label: t("digest.status.pending"),
        className: "bg-zinc-100 text-zinc-500",
      };
  }
}

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
        {paged.map((d) => {
          const badge = digestStatusBadge(d, t);
          return (
          <li key={d.id}>
            <Link
              href={`/digests/${d.id}`}
              className="flex items-center justify-between gap-3 rounded-xl border border-zinc-200 bg-white px-4 py-3 shadow-sm transition-colors hover:bg-zinc-50"
            >
              <div className="min-w-0 flex-1">
                <div className="font-medium text-zinc-900">{d.digest_date}</div>
                <div className="mt-0.5 truncate text-sm text-zinc-600">
                  {d.email_subject ??
                    t("digests.emailSubjectPending")}
                </div>
                {!!d.send_error && !d.sent_at && (
                  <div className="mt-0.5 truncate text-xs text-red-600">
                    {d.send_error}
                  </div>
                )}
                <div className="text-xs text-zinc-400">
                  {new Date(d.created_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US")}
                </div>
              </div>
              <span className={`shrink-0 rounded px-2 py-0.5 text-xs font-medium ${badge.className}`}>
                {badge.label}
              </span>
            </Link>
          </li>
          );
        })}
      </ul>
      <Pagination total={digests.length} page={page} pageSize={pageSize} onPageChange={setPage} />
    </div>
  );
}
