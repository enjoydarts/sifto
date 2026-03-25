"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { Radio } from "lucide-react";
import { useConfirm } from "@/components/confirm-provider";
import { PageTransition } from "@/components/page-transition";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { PageHeader } from "@/components/ui/page-header";
import { api, AudioBriefingDetailResponse } from "@/lib/api";
import { formatModelDisplayName } from "@/lib/model-display";

function formatDateTime(value: string | null | undefined, locale: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale === "ja" ? "ja-JP" : "en-US", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function formatDuration(seconds: number | null | undefined) {
  if (seconds == null || Number.isNaN(seconds)) return "—";
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  if (mins <= 0) return `${secs}s`;
  return `${mins}m ${secs}s`;
}

function formatSlotLabel(slotKey: string, slotStartedAt: string, locale: string, manualLabel: string) {
  if (slotKey.startsWith("manual-")) {
    return `${manualLabel} · ${formatDateTime(slotStartedAt, locale)}`;
  }
  return slotKey;
}

function formatScriptModels(value: string | null | undefined) {
  const models = (value ?? "")
    .split(",")
    .map((part) => formatModelDisplayName(part.trim()))
    .filter((part) => part.length > 0);
  if (models.length === 0) return "—";
  return models.join(", ");
}

export default function AudioBriefingDetailPage() {
  const RESUME_POLL_WINDOW_MS = 60_000;
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
  const [detail, setDetail] = useState<AudioBriefingDetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [resuming, setResuming] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [resumePending, setResumePending] = useState<{ startedAt: number; previousUpdatedAt: string | null } | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    api
      .getAudioBriefing(id)
      .then((next) => {
        if (cancelled) return;
        setDetail(next);
      })
      .catch((e) => {
        if (cancelled) return;
        setError(String(e));
      })
      .finally(() => {
        if (cancelled) return;
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  useEffect(() => {
    const status = detail?.job.status;
    const shouldPollAfterResume = resumePending != null && Date.now() - resumePending.startedAt < RESUME_POLL_WINDOW_MS;
    if ((!status || ["published", "failed", "cancelled"].includes(status)) && !shouldPollAfterResume) {
      return;
    }
    const timer = window.setInterval(() => {
      if (resumePending && Date.now() - resumePending.startedAt >= RESUME_POLL_WINDOW_MS) {
        setResumePending(null);
        return;
      }
      api
        .getAudioBriefing(id)
        .then((next) => {
          setDetail(next);
          if (
            resumePending &&
            (next.job.status !== "failed" || (next.job.updated_at ?? null) !== resumePending.previousUpdatedAt)
          ) {
            setResumePending(null);
          }
        })
        .catch(() => {});
    }, 8000);
    return () => window.clearInterval(timer);
  }, [detail?.job.status, id, resumePending]);

  const totalChars = useMemo(
    () => (detail?.chunks ?? []).reduce((sum, chunk) => sum + (chunk.char_count ?? 0), 0),
    [detail?.chunks]
  );
  const canResume = useMemo(() => {
    if (!detail) return false;
    return !!detail.resume_allowed;
  }, [detail]);
  const canDelete = useMemo(() => !!detail?.delete_allowed, [detail]);

  async function handleResume() {
    if (!detail) return;
    setResuming(true);
    try {
      const next = await api.resumeAudioBriefing(detail.job.id);
      setDetail(next);
      setResumePending({
        startedAt: Date.now(),
        previousUpdatedAt: next.job.updated_at ?? null,
      });
      showToast(t("audioBriefing.toast.resumed", "処理を再開しました"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setResuming(false);
    }
  }

  async function handleDelete() {
    if (!detail) return;
    const ok = await confirm({
      title: t("audioBriefing.deleteConfirmTitle", "この音声ブリーフィングを削除しますか？"),
      message: t("audioBriefing.deleteConfirmMessage", "台本・メタデータ・生成済み音声ファイルを削除します。この操作は元に戻せません。"),
      tone: "danger",
      confirmLabel: t("audioBriefing.deleteConfirmAction", "削除する"),
      cancelLabel: t("common.cancel", "キャンセル"),
    });
    if (!ok) return;
    setDeleting(true);
    try {
      await api.deleteAudioBriefing(detail.job.id);
      showToast(t("audioBriefing.toast.deleted", "音声ブリーフィングを削除しました"), "success");
      router.push("/audio-briefings");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeleting(false);
    }
  }

  if (loading) return <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>;
  if (error) return <p className="text-sm text-red-600">{error}</p>;
  if (!detail) return null;

  return (
    <PageTransition>
      <div className="space-y-5 overflow-x-hidden">
        <Link href="/audio-briefings" className="inline-flex items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)] hover:text-[var(--color-editorial-ink)]">
          ← {t("nav.audioBriefings", "Audio Briefings")}
        </Link>

        <PageHeader
          eyebrow={formatSlotLabel(detail.job.slot_key, detail.job.slot_started_at_jst, locale, t("audioBriefing.manualRun", "手動実行"))}
          title={detail.job.title || t("audioBriefing.untitled", "無題のエピソード")}
          titleIcon={Radio}
          description={t("audioBriefing.detailDescription", "台本生成から音声化、連結までの進行状況を確認します。")}
          actions={
            <div className="flex flex-wrap gap-2">
              {canResume ? (
                <button
                  type="button"
                  onClick={handleResume}
                  disabled={resuming}
                  className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {resuming ? t("common.saving") : t("audioBriefing.resume", "処理を再開")}
                </button>
              ) : null}
              {canDelete ? (
                <button
                  type="button"
                  onClick={handleDelete}
                  disabled={deleting}
                  className="inline-flex min-h-10 items-center rounded-full border border-[#d5bdb7] bg-[#fff4f0] px-4 py-2 text-sm font-medium text-[#7a4337] hover:bg-[#fdebe5] disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {deleting ? t("common.saving") : t("audioBriefing.delete", "削除")}
                </button>
              ) : null}
            </div>
          }
          meta={
            <div className="flex flex-wrap gap-2 text-xs">
              <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
                {t(`audioBriefing.status.${detail.job.status}`, detail.job.status)}
              </span>
              <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
                {t("audioBriefing.persona", "Persona")}: {detail.job.persona}
              </span>
              <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
                {t("audioBriefing.characters", "Chars")}: {totalChars}
              </span>
              <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
                {t("audioBriefing.duration", "Duration")}: {formatDuration(detail.job.audio_duration_sec)}
              </span>
              {detail.job.error_code ? (
                <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
                  {t("audioBriefing.errorCode", "Error code")}: {detail.job.error_code}
                </span>
              ) : null}
            </div>
          }
        />

        <section className="surface-editorial rounded-[28px] px-5 py-5">
          <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
            {t("audioBriefing.player", "Player")}
          </div>
          {detail.audio_url ? (
            <audio className="mt-4 w-full" controls src={detail.audio_url} />
          ) : (
            <div className="mt-4 rounded-[18px] border border-dashed border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-5 text-sm text-[var(--color-editorial-ink-soft)]">
              {t("audioBriefing.playerPending", "音声ファイルはまだ準備中です。台本と採用記事は先に確認できます。")}
            </div>
          )}
          <div className="mt-4 flex flex-wrap gap-x-4 gap-y-2 text-[13px] text-[var(--color-editorial-ink-soft)]">
            <span>{t("audioBriefing.createdAt", "Created")}: {formatDateTime(detail.job.created_at, locale)}</span>
            <span>{t("audioBriefing.updatedAt", "Updated")}: {formatDateTime(detail.job.updated_at, locale)}</span>
            <span>{t("audioBriefing.itemsCount", "Items")}: {detail.job.source_item_count}</span>
          </div>
          {detail.job.script_llm_models ? (
            <div className="mt-3 text-[13px] text-[var(--color-editorial-ink-soft)]">
              {t("audioBriefing.scriptModel", "Script model")}: {formatScriptModels(detail.job.script_llm_models)}
            </div>
          ) : null}
          {detail.job.error_message ? (
            <div className="mt-4 rounded-[18px] border border-[#e8cfc7] bg-[#fff4f0] px-4 py-4 text-sm leading-6 text-[#7a4337]">
              {t("audioBriefing.failureReason", "Failure")}: {detail.job.error_message}
            </div>
          ) : null}
        </section>

        <section className="surface-editorial rounded-[28px] px-5 py-5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("audioBriefing.scriptChunks", "Script Chunks")}
            </div>
            <div className="text-xs text-[var(--color-editorial-ink-soft)]">
              {detail.chunks.length} {t("common.rows")}
            </div>
          </div>
          <div className="mt-4 grid gap-3">
            {detail.chunks.map((chunk) => (
              <article key={`${chunk.seq}-${chunk.part_type}`} className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] p-4">
                <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--color-editorial-ink-soft)]">
                  <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1">
                    {chunk.seq}
                  </span>
                  <span>{chunk.part_type}</span>
                  <span>{chunk.tts_status}</span>
                  <span>{chunk.char_count} chars</span>
                </div>
                <p className="mt-3 whitespace-pre-wrap font-serif text-[17px] leading-[1.95] text-[var(--color-editorial-ink)]">{chunk.text}</p>
              </article>
            ))}
          </div>
        </section>

        <section className="surface-editorial rounded-[28px] px-5 py-5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("audioBriefing.selectedItems", "Selected Items")}
            </div>
            <div className="text-xs text-[var(--color-editorial-ink-soft)]">
              {detail.items.length} {t("common.rows")}
            </div>
          </div>
          <div className="mt-4 grid gap-3">
            {detail.items.map((item) => (
              <article key={`${item.item_id}-${item.rank}`} className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] p-4">
                <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--color-editorial-ink-soft)]">
                  <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1">
                    #{item.rank}
                  </span>
                  {item.source_title ? <span>{item.source_title}</span> : null}
                </div>
                <Link href={`/items/${item.item_id}`} className="mt-3 block text-[18px] font-semibold leading-7 text-[var(--color-editorial-ink)] hover:underline">
                  {item.translated_title || item.title || item.segment_title || t("audioBriefing.untitled", "無題")}
                </Link>
                {item.summary_snapshot ? (
                  <p className="mt-3 whitespace-pre-wrap text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{item.summary_snapshot}</p>
                ) : null}
              </article>
            ))}
          </div>
        </section>
      </div>
    </PageTransition>
  );
}
