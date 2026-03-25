"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Radio } from "lucide-react";
import { PageTransition } from "@/components/page-transition";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { PageHeader } from "@/components/ui/page-header";
import { api, AudioBriefingJob } from "@/lib/api";

function formatDateTime(value: string | null | undefined, locale: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale === "ja" ? "ja-JP" : "en-US", {
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

function formatSlotLabel(item: AudioBriefingJob, locale: string, manualLabel: string) {
  if (item.slot_key.startsWith("manual-")) {
    return `${manualLabel} · ${formatDateTime(item.slot_started_at_jst, locale)}`;
  }
  return item.slot_key;
}

function statusTone(status: string) {
  switch (status) {
    case "published":
      return "bg-[#e7f1e8] text-[#335a39]";
    case "scripted":
    case "voiced":
    case "concatenating":
    case "voicing":
      return "bg-[#eaf0f6] text-[#38506c]";
    case "skipped":
      return "bg-[#f3eee4] text-[#7b6342]";
    case "needs_rerun":
      return "bg-[#f4eee1] text-[#7a6236]";
    case "failed":
    case "cancelled":
      return "bg-[#f6e8e4] text-[#7a4337]";
    default:
      return "bg-[#f2eee7] text-[#6f6353]";
  }
}

export default function AudioBriefingsPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const [items, setItems] = useState<AudioBriefingJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    try {
      const resp = await api.getAudioBriefings({ limit: 30 });
      setItems(resp.items ?? []);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function handleGenerate() {
    setGenerating(true);
    try {
      const resp = await api.generateAudioBriefing();
      if (resp.job.status === "failed") {
        showToast(resp.job.error_message || t("audioBriefing.toast.pipelineFailed", "生成に失敗しました"), "error");
      } else {
        showToast(t("audioBriefing.toast.generated", "エピソード生成を開始しました"), "success");
      }
      setItems((prev) => {
        const next = [resp.job, ...prev.filter((item) => item.id !== resp.job.id)];
        return next;
      });
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setGenerating(false);
    }
  }

  return (
    <PageTransition>
      <div className="space-y-5 overflow-x-hidden">
        <PageHeader
          eyebrow={t("audioBriefing.eyebrow", "Audio Briefing")}
          title={t("nav.audioBriefings", "Audio Briefings")}
          titleIcon={Radio}
          description={t("audioBriefing.pageDescription", "定期生成された音声ブリーフィングと、その場で開始した手動生成を確認します。")}
          actions={
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={handleGenerate}
                disabled={generating}
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {generating ? t("common.saving") : t("audioBriefing.generateNow", "今すぐ生成")}
              </button>
              <Link
                href="/settings"
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              >
                {t("audioBriefing.openSettings", "設定を開く")}
              </Link>
            </div>
          }
        />

        {loading ? <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p> : null}
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
        {!loading && !error && items.length === 0 ? (
          <section className="surface-editorial rounded-[28px] px-5 py-5 text-sm text-[var(--color-editorial-ink-soft)]">
            {t("audioBriefing.empty", "まだエピソードがありません。")}
          </section>
        ) : null}

        <div className="grid gap-4">
          {items.map((item) => (
            <Link
              key={item.id}
              href={`/audio-briefings/${item.id}`}
              className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.78)] p-5 shadow-[var(--shadow-card)] transition-colors hover:bg-[rgba(255,253,249,0.96)]"
            >
              <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0 flex-1">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {formatSlotLabel(item, locale, t("audioBriefing.manualRun", "手動実行"))}
                  </div>
                  <h2 className="mt-3 font-serif text-[1.8rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                    {item.title || t("audioBriefing.untitled", "無題のエピソード")}
                  </h2>
                  <div className="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-[13px] text-[var(--color-editorial-ink-soft)]">
                    <span>{t("audioBriefing.persona", "Persona")}: {item.persona}</span>
                    <span>{t("audioBriefing.itemsCount", "Items")}: {item.source_item_count}</span>
                    <span>{t("audioBriefing.duration", "Duration")}: {formatDuration(item.audio_duration_sec)}</span>
                    <span>{t("audioBriefing.createdAt", "Created")}: {formatDateTime(item.created_at, locale)}</span>
                  </div>
                  {item.error_message ? (
                    <p className="mt-3 text-sm leading-6 text-[#8a4f42]">
                      {t("audioBriefing.failureReason", "Failure")}: {item.error_message}
                    </p>
                  ) : null}
                </div>
                <span className={`shrink-0 rounded-full px-3 py-2 text-xs font-semibold ${statusTone(item.status)}`}>
                  {t(`audioBriefing.status.${item.status}`, item.status)}
                </span>
              </div>
            </Link>
          ))}
        </div>
      </div>
    </PageTransition>
  );
}
