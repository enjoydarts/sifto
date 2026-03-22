"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Mail } from "lucide-react";
import Pagination from "@/components/pagination";
import { PageTransition } from "@/components/page-transition";
import { useI18n } from "@/components/i18n-provider";
import { PageHeader } from "@/components/ui/page-header";
import { api, Digest } from "@/lib/api";

function digestStatusBadge(d: Digest, t: (key: string, fallback?: string) => string) {
  if (d.sent_at) {
    return {
      label: t("digest.status.sent"),
      className: "bg-[#e7f1e8] text-[#335a39]",
    };
  }
  switch (d.send_status) {
    case "compose_failed":
    case "send_email_failed":
    case "fetch_failed":
    case "user_key_failed":
      return {
        label: t("digest.status.failed"),
        className: "bg-[#f6e8e4] text-[#7a4337]",
      };
    case "processing":
      return {
        label: t("digest.status.processing"),
        className: "bg-[#eaf0f6] text-[#38506c]",
      };
    case "skipped_resend_disabled":
      return {
        label: t("digest.status.resendDisabled"),
        className: "bg-[#f3eee4] text-[#7b6342]",
      };
    case "skipped_user_disabled":
      return {
        label: t("digest.status.emailDisabled"),
        className: "bg-[#f2eee7] text-[#6f6353]",
      };
    case "skipped_no_items":
      return {
        label: t("digest.status.noItems"),
        className: "bg-[#f2eee7] text-[#6f6353]",
      };
    default:
      return {
        label: t("digest.status.pending"),
        className: "bg-[#f2eee7] text-[#6f6353]",
      };
  }
}

function formatDateTime(value: string | null | undefined, locale: string) {
  if (!value) return null;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return new Intl.DateTimeFormat(locale === "ja" ? "ja-JP" : "en-US", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

function buildIssueNote(digest: Digest, t: (key: string, fallback?: string) => string) {
  const source = (digest.email_body ?? digest.send_error ?? "").replace(/\s+/g, " ").trim();
  if (!source) return t("digests.issueNoteFallback");
  const cleaned = source
    .replace(/[#>*_`-]/g, " ")
    .replace(/\[(.*?)\]\((.*?)\)/g, "$1")
    .replace(/\s+/g, " ")
    .trim();
  if (cleaned.length <= 220) return cleaned;
  return `${cleaned.slice(0, 220).trim()}...`;
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
  const sentCount = useMemo(() => digests.filter((digest) => !!digest.sent_at).length, [digests]);
  const processingCount = useMemo(() => digests.filter((digest) => digest.send_status === "processing").length, [digests]);
  const failedCount = useMemo(
    () =>
      digests.filter((digest) =>
        ["compose_failed", "send_email_failed", "fetch_failed", "user_key_failed"].includes(digest.send_status ?? "")
      ).length,
    [digests]
  );
  const newestSentAt = useMemo(() => {
    const values = digests
      .map((digest) => digest.sent_at)
      .filter((value): value is string => typeof value === "string")
      .sort((a, b) => new Date(b).getTime() - new Date(a).getTime());
    return values[0] ?? null;
  }, [digests]);

  return (
    <PageTransition>
      <div className="space-y-5 overflow-x-hidden">
        <PageHeader
          eyebrow="Digest Archive"
          title={t("nav.digests")}
          titleIcon={Mail}
          description={t("digests.archiveSubtitle")}
          meta={
            <div className="flex flex-wrap gap-2 text-xs">
              <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
                {digests.length.toLocaleString()} {t("digests.metaIssues")}
              </span>
              {newestSentAt ? (
                <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
                  {t("digests.metaNewest")} {formatDateTime(newestSentAt, locale)}
                </span>
              ) : null}
              {processingCount > 0 ? (
                <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
                  {processingCount} {t("digests.metaProcessing")}
                </span>
              ) : null}
            </div>
          }
        />

        {loading && <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>}
        {error && <p className="text-sm text-red-600">{error}</p>}
        {!loading && digests.length === 0 && <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("digests.empty")}</p>}

        {!loading && digests.length > 0 ? (
          <>
            <section className="surface-editorial rounded-[28px] px-5 py-5">
              <div className="grid gap-3 md:grid-cols-3">
                <ArchiveMetric label={t("digests.metricSent")} value={sentCount.toLocaleString()} />
                <ArchiveMetric label={t("digests.metricProcessing")} value={processingCount.toLocaleString()} />
                <ArchiveMetric label={t("digests.metricFailed")} value={failedCount.toLocaleString()} />
              </div>
            </section>

            <ul className="grid gap-4">
              {paged.map((digest) => {
                const badge = digestStatusBadge(digest, t);
                const note = buildIssueNote(digest, t);
                return (
                  <li key={digest.id}>
                    <Link
                      href={`/digests/${digest.id}`}
                      className="block rounded-[28px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.78)] p-5 shadow-[var(--shadow-card)] transition-colors hover:bg-[rgba(255,253,249,0.96)]"
                    >
                      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                        <div className="min-w-0 flex-1">
                          <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                            {digest.digest_date}
                          </div>
                          <h2 className="mt-3 font-serif text-[28px] leading-[1.2] tracking-[-0.03em] text-[var(--color-editorial-ink)] sm:text-[34px]">
                            {digest.email_subject ?? t("digests.emailSubjectPending")}
                          </h2>
                          <div className="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-[13px] text-[var(--color-editorial-ink-soft)]">
                            <span>
                              {t("digests.createdAt")} {formatDateTime(digest.created_at, locale)}
                            </span>
                            {digest.sent_at ? (
                              <span>
                                {t("digests.sentAt")} {formatDateTime(digest.sent_at, locale)}
                              </span>
                            ) : null}
                            <span>
                              {t("digests.retryMeta")} {digest.digest_retry_count}
                            </span>
                          </div>
                        </div>
                        <span className={`shrink-0 rounded-full px-3 py-2 text-xs font-semibold ${badge.className}`}>{badge.label}</span>
                      </div>

                      <div className="mt-5 grid gap-4 lg:grid-cols-[minmax(0,1fr)_260px]">
                        <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] px-4 py-4">
                          <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                            {t("digests.issueNote")}
                          </div>
                          <p className="mt-3 text-sm leading-8 text-[var(--color-editorial-ink-soft)]">{note}</p>
                        </div>
                        <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] px-4 py-4">
                          <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                            {t("digests.archiveTrail")}
                          </div>
                          <div className="mt-3 text-sm font-semibold text-[var(--color-editorial-ink)]">
                            {t("digests.trailStatus")}
                          </div>
                          <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                            {digest.send_error ? t("digests.trailFailed") : digest.sent_at ? t("digests.trailSent") : t("digests.trailPending")}
                          </p>
                        </div>
                      </div>

                      {!!digest.send_error && !digest.sent_at ? (
                        <div className="mt-4 border-t border-[var(--color-editorial-line)] pt-4 text-sm text-[#7a4337]">
                          {digest.send_error}
                        </div>
                      ) : null}
                    </Link>
                  </li>
                );
              })}
            </ul>

            <Pagination total={digests.length} page={page} pageSize={pageSize} onPageChange={setPage} />
          </>
        ) : null}
      </div>
    </PageTransition>
  );
}

function ArchiveMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.72)] px-4 py-4 shadow-[var(--shadow-card)]">
      <div className="text-[11px] uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{label}</div>
      <div className="mt-2 font-serif text-[30px] leading-none text-[var(--color-editorial-ink)]">{value}</div>
    </div>
  );
}
