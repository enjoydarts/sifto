"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { Radio } from "lucide-react";
import { useConfirm } from "@/components/confirm-provider";
import { PageTransition } from "@/components/page-transition";
import { useSharedAudioPlayer } from "@/components/shared-audio-player/provider";
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

function formatUsedTTSModel(value: string | null | undefined) {
  const raw = (value ?? "").trim();
  if (!raw) return "—";
  return formatModelDisplayName(raw);
}

function formatUsedVoiceLabel(label: string | null | undefined, model: string | null | undefined) {
  const normalizedLabel = (label ?? "").trim();
  if (normalizedLabel) return normalizedLabel;
  const normalizedModel = (model ?? "").trim();
  if (normalizedModel) return normalizedModel;
  return "—";
}

function formatBGMName(value: string | null | undefined) {
  const raw = (value ?? "").trim();
  if (!raw) return "—";
  const parts = raw.split("/");
  const filename = parts[parts.length - 1] || raw;
  return filename.replace(/\.[^.]+$/, "");
}

function formatConversationMode(mode: string | null | undefined, t: (key: string, fallback?: string) => string) {
  const normalized = mode === "duo" ? "duo" : "single";
  return t(`audioBriefing.conversationMode.${normalized}`, normalized);
}

function llmPromptSourceLabel(source: string | null | undefined, version: number | null | undefined, t: (key: string, fallback?: string) => string) {
  const normalized = (source ?? "").trim();
  if (normalized === "default_code") return t("itemDetail.execution.prompt.defaultCode");
  if (normalized === "template_version" && version != null) {
    return t("itemDetail.execution.prompt.templateVersion").replace("{{version}}", String(version));
  }
  if (normalized === "template_version") return t("itemDetail.execution.prompt.template");
  return normalized || null;
}

function formatChunkSpeaker(
  speaker: string | null | undefined,
  personaDefinitions: Record<string, { name: string }> | undefined,
  detail: AudioBriefingDetailResponse,
  t: (key: string, fallback?: string) => string
) {
  if (speaker === "host") {
    return resolvePersonaCharacterName(detail.job.persona, personaDefinitions) || t("audioBriefing.speaker.host", "Host");
  }
  if (speaker === "partner") {
    return resolvePersonaCharacterName(detail.job.partner_persona, personaDefinitions) || t("audioBriefing.speaker.partner", "Partner");
  }
  return null;
}

function resolvePersonaCharacterName(
  persona: string | null | undefined,
  personaDefinitions?: Record<string, { name: string }>
) {
  const key = (persona ?? "").trim();
  if (!key) return null;
  return personaDefinitions?.[key]?.name?.trim() || key;
}

function formatArticleChunkOrdinal(
  chunk: AudioBriefingDetailResponse["chunks"][number],
  chunkIndex: number,
  detail: AudioBriefingDetailResponse,
  locale: string,
  t: (key: string, fallback?: string) => string
) {
  if (chunk.part_type !== "article") return null;
  const articleChunksBefore = detail.chunks
    .slice(0, chunkIndex + 1)
    .filter((candidate) => candidate.part_type === "article");
  let ordinal = articleChunksBefore.length;
  if (detail.job.conversation_mode === "duo") {
    ordinal = 0;
    let previousSpeaker: string | null = null;
    for (const articleChunk of articleChunksBefore) {
      const speaker = articleChunk.speaker ?? null;
      if (ordinal === 0) {
        ordinal = 1;
      } else if (speaker === "host" && previousSpeaker === "host") {
        ordinal += 1;
      }
      previousSpeaker = speaker;
    }
  }
  if (locale === "ja") {
    return `${ordinal}${t("audioBriefing.articleOrdinalSuffix", "本目の記事")}`;
  }
  return `${t("audioBriefing.partType.article", "Article")} ${ordinal}`;
}

function formatPipelineStage(stage: string | null | undefined, t: (key: string, fallback?: string) => string) {
  if (!stage) return "—";
  return t(`audioBriefing.pipelineStage.${stage}`, stage);
}

function formatChunkPartType(partType: string, t: (key: string, fallback?: string) => string) {
  switch (partType) {
    case "opening":
      return t("audioBriefing.partType.opening", "Opening");
    case "summary":
      return t("audioBriefing.partType.summary", "Summary");
    case "article":
      return t("audioBriefing.partType.article", "Article");
    case "ending":
      return t("audioBriefing.partType.ending", "Ending");
    default:
      return partType;
  }
}

function audioBriefingMessageLabel(status: string | null | undefined, t: (key: string, fallback?: string) => string) {
  if (status === "skipped") {
    return t("audioBriefing.skipReason", "Skip reason");
  }
  return t("audioBriefing.failureReason", "Failure reason");
}

export default function AudioBriefingDetailPage() {
  const RESUME_POLL_WINDOW_MS = 60_000;
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const player = useSharedAudioPlayer();
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
  const latestSessionsQuery = useQuery({
    queryKey: ["latest-playback-sessions"],
    queryFn: () => api.getLatestPlaybackSessions(),
  });
  const navigatorPersonasQuery = useQuery({
    queryKey: ["navigator-personas"],
    queryFn: () => api.getNavigatorPersonas(),
  });
  const latestAudioSession = latestSessionsQuery.data?.audio_briefing ?? null;
  const [detail, setDetail] = useState<AudioBriefingDetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [resuming, setResuming] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const [unarchiving, setUnarchiving] = useState(false);
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
  const canArchive = useMemo(() => !!detail?.archive_allowed, [detail]);
  const canUnarchive = useMemo(() => !!detail?.unarchive_allowed, [detail]);

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

  async function handleArchive() {
    if (!detail) return;
    setArchiving(true);
    try {
      const next = await api.archiveAudioBriefing(detail.job.id);
      setDetail(next);
      showToast(t("audioBriefing.toast.archived", "アーカイブしました"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setArchiving(false);
    }
  }

  async function handleUnarchive() {
    if (!detail) return;
    setUnarchiving(true);
    try {
      const next = await api.unarchiveAudioBriefing(detail.job.id);
      setDetail(next);
      showToast(t("audioBriefing.toast.unarchived", "公開中に戻しました"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setUnarchiving(false);
    }
  }

  async function handlePlayInSharedPlayer() {
    if (!detail?.audio_url) {
      return;
    }
    await player.startAudioBriefingPlayback({
      jobID: detail.job.id,
      title: detail.job.title || t("audioBriefing.untitled", "無題のエピソード"),
      summary: t("audioBriefing.detailDescription", "台本生成から音声化、連結までの進行状況を確認します。"),
      audioURL: detail.audio_url,
      detailHref: `/audio-briefings/${detail.job.id}`,
    });
    player.expandPlayer();
  }

  async function handleResumeLatestPlayback() {
    if (!latestAudioSession) {
      return;
    }
    await player.resumePlaybackSession(latestAudioSession);
    player.expandPlayer();
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
              {canArchive ? (
                <button
                  type="button"
                  onClick={handleArchive}
                  disabled={archiving}
                  className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {archiving ? t("common.saving") : t("audioBriefing.archive", "アーカイブ")}
                </button>
              ) : null}
              {canUnarchive ? (
                <button
                  type="button"
                  onClick={handleUnarchive}
                  disabled={unarchiving}
                  className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {unarchiving ? t("common.saving") : t("audioBriefing.unarchive", "公開中に戻す")}
                </button>
              ) : null}
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
        />

        <div className="flex flex-wrap gap-2 text-xs">
          <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
            {t(`audioBriefing.status.${detail.job.status}`, detail.job.status)}
          </span>
          <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
            {t("audioBriefing.hostPersona", "Host")}: {resolvePersonaCharacterName(detail.job.persona, navigatorPersonasQuery.data)}
          </span>
          {detail.job.partner_persona ? (
            <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
              {t("audioBriefing.partnerPersona", "Partner")}: {resolvePersonaCharacterName(detail.job.partner_persona, navigatorPersonasQuery.data)}
            </span>
          ) : null}
          <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
            {t("audioBriefing.conversationMode", "Conversation")}: {formatConversationMode(detail.job.conversation_mode, t)}
          </span>
          {detail.job.pipeline_stage ? (
            <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)]">
              {t("audioBriefing.pipelineStage", "Pipeline")}: {formatPipelineStage(detail.job.pipeline_stage, t)}
            </span>
          ) : null}
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

        <section className="surface-editorial rounded-[28px] px-5 py-5">
          <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
            {t("audioBriefing.player", "Player")}
          </div>
          {detail.audio_url ? (
            <div className="mt-4 flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => void handlePlayInSharedPlayer()}
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
              >
                {t("audioBriefing.playInSharedPlayer", "プレイヤーで再生")}
              </button>
              {latestAudioSession ? (
                <button
                  type="button"
                  onClick={() => void handleResumeLatestPlayback()}
                  className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                >
                  {t("playbackHistory.resumeLatest")}
                </button>
              ) : null}
              <Link
                href="/playback-history"
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              >
                {t("nav.playbackHistory")}
              </Link>
              <button
                type="button"
                onClick={player.expandPlayer}
                disabled={player.mode !== "audio_briefing" || player.audioBriefing?.jobID !== detail.job.id}
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-60"
              >
                {t("audioBriefing.openPlayerOverlay", "プレイヤーを開く")}
              </button>
            </div>
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
            <div className="mt-3 flex flex-wrap items-center gap-2 text-[13px] text-[var(--color-editorial-ink-soft)]">
              <span>{t("audioBriefing.scriptModel", "Script model")}: {formatScriptModels(detail.job.script_llm_models)}</span>
              {llmPromptSourceLabel(detail.job.prompt_source, detail.job.prompt_version_number, t) ? (
                <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-1 text-xs">
                  {llmPromptSourceLabel(detail.job.prompt_source, detail.job.prompt_version_number, t)}
                </span>
              ) : null}
              {detail.job.prompt_experiment_id ? (
                <span className="rounded-full border border-amber-200 bg-amber-50 px-2 py-1 text-xs text-amber-700">
                  {t("itemDetail.execution.prompt.experiment")}
                </span>
              ) : null}
              {detail.job.prompt_key ? (
                <span className="text-xs">{t("itemDetail.execution.prompt.key").replace("{{key}}", detail.job.prompt_key)}</span>
              ) : null}
            </div>
          ) : null}
          {detail.used_tts ? (
            <div className="mt-3 flex flex-wrap items-center gap-2 text-[13px] text-[var(--color-editorial-ink-soft)]">
              <span>{t("audioBriefing.ttsProvider", "TTS provider")}: {detail.used_tts.provider || "—"}</span>
              <span>{t("audioBriefing.ttsModel", "TTS model")}: {formatUsedTTSModel(detail.used_tts.tts_model)}</span>
              <span>{t("audioBriefing.hostVoiceModel", "Host voice")}: {formatUsedVoiceLabel(detail.used_tts.host_voice_label, detail.used_tts.host_voice_model)}</span>
              {detail.job.conversation_mode === "duo" ? (
                <span>{t("audioBriefing.partnerVoiceModel", "Partner voice")}: {formatUsedVoiceLabel(detail.used_tts.partner_voice_label, detail.used_tts.partner_voice_model)}</span>
              ) : null}
            </div>
          ) : null}
          {detail.job.bgm_object_key ? (
            <div className="mt-2 text-[13px] text-[var(--color-editorial-ink-soft)]">
              {t("audioBriefing.bgmTrack", "BGM")}: {formatBGMName(detail.job.bgm_object_key)}
            </div>
          ) : null}
          {detail.job.error_message ? (
            <div className="mt-4 rounded-[18px] border border-[#e8cfc7] bg-[#fff4f0] px-4 py-4 text-sm leading-6 text-[#7a4337]">
              {audioBriefingMessageLabel(detail.job.status, t)}: {detail.job.error_message}
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
            {detail.chunks.map((chunk, chunkIndex) => (
              <article key={`${chunk.seq}-${chunk.part_type}`} className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] p-4">
                <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--color-editorial-ink-soft)]">
                  <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1">
                    {chunk.seq}
                  </span>
                  <span>{formatChunkPartType(chunk.part_type, t)}</span>
                  {formatArticleChunkOrdinal(chunk, chunkIndex, detail, locale, t) ? (
                    <span>{formatArticleChunkOrdinal(chunk, chunkIndex, detail, locale, t)}</span>
                  ) : null}
                  {formatChunkSpeaker(chunk.speaker, navigatorPersonasQuery.data, detail, t) ? (
                    <span>
                      {t("audioBriefing.speaker", "Speaker")}: {formatChunkSpeaker(chunk.speaker, navigatorPersonasQuery.data, detail, t)}
                    </span>
                  ) : null}
                  <span>{chunk.tts_status}</span>
                  <span>{chunk.char_count} chars</span>
                </div>
                <p className="mt-3 whitespace-pre-wrap font-serif text-[17px] leading-[1.95] text-[var(--color-editorial-ink)]">{chunk.text}</p>
                {chunk.preprocessed_text ? (
                  <details className="mt-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3">
                    <summary className="cursor-pointer text-sm font-medium text-[var(--color-editorial-ink)]">
                      {t("audioBriefing.preprocessedTextLabel", "Fish前処理テキスト")}
                    </summary>
                    <p className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">
                      {t("audioBriefing.preprocessedTextHelp", "Fish Audio に送る直前の前処理済みテキストを確認できます。")}
                    </p>
                    <pre className="mt-3 whitespace-pre-wrap break-words rounded-[16px] bg-[var(--color-editorial-panel-strong)] px-3 py-3 text-sm leading-6 text-[var(--color-editorial-ink)] [overflow-wrap:anywhere]">
                      {chunk.preprocessed_text}
                    </pre>
                  </details>
                ) : null}
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
