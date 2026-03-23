"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { Send } from "lucide-react";
import { PageTransition } from "@/components/page-transition";
import { useI18n } from "@/components/i18n-provider";
import { api, DigestDetail } from "@/lib/api";

function digestStatusBadge(d: DigestDetail, t: (key: string, fallback?: string) => string) {
  if (d.sent_at) {
    return {
      label: t("digest.status.sent"),
      className: "bg-[#e7f1e8] text-[#335a39]",
      withSendIcon: true,
    };
  }
  switch (d.send_status) {
    case "compose_failed":
    case "send_email_failed":
    case "fetch_failed":
    case "user_key_failed":
      return { label: t("digest.status.failed"), className: "bg-[#f6e8e4] text-[#7a4337]", withSendIcon: false };
    case "processing":
      return { label: t("digest.status.processing"), className: "bg-[#eaf0f6] text-[#38506c]", withSendIcon: false };
    case "skipped_resend_disabled":
      return { label: t("digest.status.resendDisabled"), className: "bg-[#f3eee4] text-[#7b6342]", withSendIcon: false };
    case "skipped_user_disabled":
      return { label: t("digest.status.emailDisabled"), className: "bg-[#f2eee7] text-[#6f6353]", withSendIcon: false };
    case "skipped_no_items":
      return { label: t("digest.status.noItems"), className: "bg-[#f2eee7] text-[#6f6353]", withSendIcon: false };
    default:
      return { label: t("digest.status.pending"), className: "bg-[#f2eee7] text-[#6f6353]", withSendIcon: false };
  }
}

function formatDateTime(value: string | null | undefined, locale: string) {
  if (!value) return "—";
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

  const statusBadge = useMemo(() => (digest ? digestStatusBadge(digest, t) : null), [digest, t]);

  if (loading) return <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>;
  if (error) return <p className="text-sm text-red-600">{error}</p>;
  if (!digest || !statusBadge) return null;

  return (
    <PageTransition>
      <div className="min-w-0 space-y-5 overflow-x-hidden">
        <Link href="/digests" className="inline-flex items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)] hover:text-[var(--color-editorial-ink)]">
          ← {t("nav.digests")}
        </Link>

        <section
          className="rounded-[30px] border border-[var(--color-editorial-line)] px-5 py-5 shadow-[var(--shadow-card)] sm:px-6"
          style={{
            background: "linear-gradient(180deg, rgba(255,255,255,0.72), rgba(255,253,249,0.96)), #fbf8f2",
          }}
        >
          <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
            {digest.digest_date}
          </div>
          <h1 className="mt-3 font-serif text-[2.25rem] leading-[1.06] tracking-[-0.04em] text-[var(--color-editorial-ink)] sm:text-[3.2rem]">
            {digest.email_subject ?? t("digests.emailSubjectPending")}
          </h1>
          <div className="mt-4 flex flex-wrap gap-2 text-xs">
            <span className={`inline-flex items-center gap-1 rounded-full px-3 py-2 font-semibold ${statusBadge.className}`}>
              {statusBadge.withSendIcon ? <Send className="size-3.5" aria-hidden="true" /> : null}
              {statusBadge.label}
            </span>
            <MetaPill>{t("digestDetail.createdAt")} {formatDateTime(digest.created_at, locale)}</MetaPill>
            <MetaPill>{t("digestDetail.sentAt")} {formatDateTime(digest.sent_at, locale)}</MetaPill>
            <MetaPill>{t("digestDetail.digestRetryCount")} {digest.digest_retry_count}</MetaPill>
            <MetaPill>{t("digestDetail.clusterDraftRetryCount")} {digest.cluster_draft_retry_count}</MetaPill>
          </div>
          {!!digest.send_error && !digest.sent_at ? (
            <div className="mt-5 border-t border-[var(--color-editorial-line)] pt-4 text-sm leading-7 text-[#7a4337]">
              {digest.send_error}
            </div>
          ) : null}
        </section>

        {digest.email_body ? (
          <section className="surface-editorial rounded-[28px] px-5 py-5 sm:px-6">
            <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("digestDetail.emailBody")}
            </div>
            {digest.digest_llm ? (
              <div className="mt-3 text-xs text-[var(--color-editorial-ink-soft)]" title={t("digestDetail.digestModelTitle")}>
                {digest.digest_llm.provider} / {digest.digest_llm.model}
              </div>
            ) : null}
            <div className="mt-4 whitespace-pre-wrap break-words font-serif text-[18px] leading-[1.95] text-[var(--color-editorial-ink)]">
              {digest.email_body}
            </div>
          </section>
        ) : null}

        {digest.cluster_drafts && digest.cluster_drafts.length > 0 ? (
          <section className="surface-editorial rounded-[28px] px-5 py-5 sm:px-6">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                {t("digestDetail.clusterDrafts")}
              </div>
              <div className="text-xs text-[var(--color-editorial-ink-soft)]">
                {digest.cluster_drafts.length} {t("digestDetail.clusters")}
              </div>
            </div>
            {digest.cluster_draft_llm ? (
              <div className="mt-3 text-xs text-[var(--color-editorial-ink-soft)]" title={t("digestDetail.clusterDraftModelTitle")}>
                {digest.cluster_draft_llm.provider} / {digest.cluster_draft_llm.model}
              </div>
            ) : null}
            <div className="mt-4 grid gap-3">
              {digest.cluster_drafts.map((draft) => (
                <article key={draft.id} className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] p-4">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-soft)]">
                      #{draft.rank}
                    </span>
                    <span className="text-sm font-semibold text-[var(--color-editorial-ink)]">{draft.cluster_label}</span>
                    <span className="text-xs text-[var(--color-editorial-ink-soft)]">
                      {draft.item_count} {t("digestDetail.itemsUnit")}
                    </span>
                    {typeof draft.max_score === "number" ? (
                      <span className="text-xs text-[var(--color-editorial-ink-soft)]">score {draft.max_score.toFixed(2)}</span>
                    ) : null}
                  </div>
                  <div className="mt-3 whitespace-pre-wrap break-words font-serif text-[17px] leading-[1.95] text-[var(--color-editorial-ink)]">
                    {draft.draft_summary}
                  </div>
                </article>
              ))}
            </div>
          </section>
        ) : null}

        <section className="surface-editorial rounded-[28px] px-5 py-5 sm:px-6">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("digestDetail.items")}
            </div>
            <div className="text-xs text-[var(--color-editorial-ink-soft)]">
              {digest.items.length} {t("common.rows")}
            </div>
          </div>

          {digest.items.length === 0 ? (
            <p className="mt-4 text-sm text-[var(--color-editorial-ink-faint)]">{t("digestDetail.noItems")}</p>
          ) : (
            <div className="mt-4 grid gap-3">
              {digest.items.map((di) => (
                <article key={di.item.id} className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] p-4">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-soft)]">
                      #{di.rank}
                    </span>
                    {typeof di.summary.score === "number" ? (
                      <span className="text-xs text-[var(--color-editorial-ink-soft)]">score {di.summary.score.toFixed(2)}</span>
                    ) : null}
                  </div>
                  <Link href={`/items/${di.item.id}`} className="mt-3 block text-[18px] font-semibold leading-7 text-[var(--color-editorial-ink)] hover:underline">
                    {di.item.title ?? di.item.url}
                  </Link>
                  <a
                    href={di.item.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="mt-1 block break-all text-xs text-[var(--color-editorial-ink-faint)] hover:text-[var(--color-editorial-ink-soft)]"
                  >
                    {di.item.url}
                  </a>
                  <p className="mt-3 whitespace-pre-wrap font-serif text-[17px] leading-[1.95] text-[var(--color-editorial-ink)]">
                    {di.summary.summary}
                  </p>
                  {di.summary.topics.length > 0 ? (
                    <div className="mt-3 flex flex-wrap gap-2">
                      {di.summary.topics.map((topic) => (
                        <span
                          key={topic}
                          className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[11px] text-[var(--color-editorial-ink-soft)]"
                        >
                          {topic}
                        </span>
                      ))}
                    </div>
                  ) : null}
                </article>
              ))}
            </div>
          )}
        </section>
      </div>
    </PageTransition>
  );
}

function MetaPill({ children }: { children: React.ReactNode }) {
  return (
    <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-[var(--color-editorial-ink-soft)]">
      {children}
    </span>
  );
}
