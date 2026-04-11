"use client";

import type { FormEvent } from "react";
import Link from "next/link";
import AudioBriefingVoiceRow from "@/components/settings/sections/audio-briefing-voice-row";
import type {
  AudioBriefingNumericInputField,
  AudioBriefingVoiceInputDrafts,
  AudioBriefingVoiceSummary,
  ModelSelectLabels,
  Translate,
} from "@/components/settings/sections/audio-briefing-settings-types";
import { SectionCard } from "@/components/ui/section-card";
import type {
  AivisModelSnapshot,
  AudioBriefingPersonaVoice,
  ElevenLabsVoiceCatalogEntry,
  GeminiTTSVoiceCatalogEntry,
  OpenAITTSVoiceSnapshot,
  XAIVoiceSnapshot,
} from "@/lib/api";

export default function AudioBriefingVoiceMatrixSection({
  t,
  modelSelectLabels,
  form,
  status,
  availability,
  catalogs,
  actions,
}: {
  t: Translate;
  modelSelectLabels: ModelSelectLabels;
  form: {
    onSubmit: (event: FormEvent<HTMLFormElement>) => void;
    saving: boolean;
    onPersistAudioBriefingVoices: () => void;
  };
  status: {
    readyCount: number;
    attentionCount: number;
    configuredCount: number;
    totalCount: number;
    aivisModelsError: string | null;
    xaiVoicesError: string | null;
    elevenLabsVoicesError: string | null;
    geminiTTSVoicesError: string | null;
    needsAivisAPIKey: boolean;
    needsXAIAPIKey: boolean;
    needsFishAPIKey: boolean;
    needsElevenLabsAPIKey: boolean;
    needsOpenAIAPIKey: boolean;
    needsGeminiAccess: boolean;
    aivisLatestSyncedAt?: string;
    openAITTSLatestSyncedAt?: string;
  };
  availability: {
    voiceSummaries: AudioBriefingVoiceSummary[];
    expandedPersonas: string[];
    defaultPersona: string;
    conversationMode: "single" | "duo";
    hasUserFishAPIKey: boolean;
    hasUserXAIAPIKey: boolean;
    hasUserOpenAIAPIKey: boolean;
    hasUserElevenLabsAPIKey: boolean;
    geminiTTSEnabled: boolean;
  };
  catalogs: {
    audioBriefingAivisModels: AivisModelSnapshot[];
    audioBriefingXAIVoices: XAIVoiceSnapshot[];
    audioBriefingOpenAITTSVoices: OpenAITTSVoiceSnapshot[];
    audioBriefingGeminiTTSVoices: GeminiTTSVoiceCatalogEntry[];
    audioBriefingElevenLabsVoices: ElevenLabsVoiceCatalogEntry[];
    audioBriefingVoiceInputDrafts: AudioBriefingVoiceInputDrafts;
    aivisModelsSyncing: boolean;
    xaiVoicesSyncing: boolean;
    openAITTSVoicesSyncing: boolean;
    geminiTTSVoicesLoading: boolean;
  };
  actions: {
    onOpenSystemForProvider: (provider: string) => void;
    onSyncAivisModels: () => void;
    onTogglePersona: (persona: string) => void;
    onUpdateVoice: (persona: string, patch: Partial<AudioBriefingPersonaVoice>) => void;
    onOpenAivisPicker: (persona: string) => void;
    onOpenFishPicker: (persona: string) => void;
    onOpenXAIPicker: (persona: string) => void;
    onOpenOpenAITTSPicker: (persona: string) => void;
    onOpenGeminiTTSPicker: (persona: string) => void;
    onOpenElevenLabsPicker: (persona: string) => void;
    onUpdateVoiceNumberInput: (persona: string, field: AudioBriefingNumericInputField, raw: string) => void;
    onResetVoiceNumberInput: (persona: string, field: AudioBriefingNumericInputField) => void;
    onSyncXAIVoices: () => void;
    onSyncOpenAITTSVoices: () => void;
    onLoadGeminiTTSVoices: () => void;
  };
}) {
  const { onSubmit, saving, onPersistAudioBriefingVoices } = form;
  const {
    readyCount,
    attentionCount,
    configuredCount,
    totalCount,
    aivisModelsError,
    xaiVoicesError,
    elevenLabsVoicesError,
    geminiTTSVoicesError,
    needsAivisAPIKey,
    needsXAIAPIKey,
    needsFishAPIKey,
    needsElevenLabsAPIKey,
    needsOpenAIAPIKey,
    needsGeminiAccess,
    aivisLatestSyncedAt,
    openAITTSLatestSyncedAt,
  } = status;
  const {
    voiceSummaries,
    expandedPersonas,
    defaultPersona,
    conversationMode,
    hasUserFishAPIKey,
    hasUserXAIAPIKey,
    hasUserOpenAIAPIKey,
    hasUserElevenLabsAPIKey,
    geminiTTSEnabled,
  } = availability;
  const {
    audioBriefingAivisModels,
    audioBriefingXAIVoices,
    audioBriefingOpenAITTSVoices,
    audioBriefingGeminiTTSVoices,
    audioBriefingElevenLabsVoices,
    audioBriefingVoiceInputDrafts,
    aivisModelsSyncing,
    xaiVoicesSyncing,
    openAITTSVoicesSyncing,
    geminiTTSVoicesLoading,
  } = catalogs;
  const {
    onOpenSystemForProvider,
    onSyncAivisModels,
    onTogglePersona,
    onUpdateVoice,
    onOpenAivisPicker,
    onOpenFishPicker,
    onOpenXAIPicker,
    onOpenOpenAITTSPicker,
    onOpenGeminiTTSPicker,
    onOpenElevenLabsPicker,
    onUpdateVoiceNumberInput,
    onResetVoiceNumberInput,
    onSyncXAIVoices,
    onSyncOpenAITTSVoices,
    onLoadGeminiTTSVoices,
  } = actions;
  const renderWarning = (
    show: boolean,
    title: string,
    detail: string,
    provider?: string,
  ) =>
    show ? (
      <div className="flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
        <div>
          <div className="font-semibold">{title}</div>
          <div className="mt-1 leading-6">{detail}</div>
        </div>
        {provider ? (
          <button
            type="button"
            onClick={() => onOpenSystemForProvider(provider)}
            className="inline-flex min-h-10 items-center justify-center rounded-full border border-[rgba(180,83,9,0.22)] bg-white px-4 py-2 text-sm font-medium text-[#92400e] hover:bg-[rgba(255,255,255,0.72)]"
          >
            {t("settings.audioBriefing.openApiKeys")}
          </button>
        ) : null}
      </div>
    ) : null;

  return (
    <SectionCard>
      <form onSubmit={onSubmit} className="space-y-4">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.voiceMatrixTitle")}</div>
            <div className="mt-1 flex flex-wrap items-center gap-3 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">
              <p>{t("settings.audioBriefing.voiceMatrixHelp")}</p>
              <Link href="/aivis-models" className="font-medium text-[var(--color-editorial-accent)] underline-offset-4 hover:underline">
                {t("settings.audioBriefing.openAivisModels")}
              </Link>
              <Link href="/openai-tts-voices" className="font-medium text-[var(--color-editorial-accent)] underline-offset-4 hover:underline">
                {t("settings.audioBriefing.openOpenAITTSVoices")}
              </Link>
              <Link href="/gemini-tts-voices" className="font-medium text-[var(--color-editorial-accent)] underline-offset-4 hover:underline">
                {t("settings.audioBriefing.openGeminiTTSVoices")}
              </Link>
              {aivisLatestSyncedAt ? <span>{`${t("aivisModels.lastSynced")}: ${new Date(aivisLatestSyncedAt).toLocaleString()}`}</span> : null}
              {openAITTSLatestSyncedAt ? <span>{`${t("openaiTTS.lastSynced")}: ${new Date(openAITTSLatestSyncedAt).toLocaleString()}`}</span> : null}
            </div>
          </div>
          <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
            <button
              type="button"
              onClick={onSyncAivisModels}
              disabled={aivisModelsSyncing}
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
            >
              {aivisModelsSyncing ? t("aivisModels.syncing") : t("aivisModels.sync")}
            </button>
            <button
              type="submit"
              disabled={saving}
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {saving ? t("common.saving") : t("settings.audioBriefing.saveVoices")}
            </button>
          </div>
        </div>

        <div className="flex flex-wrap gap-3">
          <div className="min-w-[180px] flex-1 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.audioBriefing.summary.ready")}
            </div>
            <div className="mt-2 text-2xl font-semibold text-[var(--color-editorial-ink)]">{readyCount}</div>
          </div>
          <div className="min-w-[180px] flex-1 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.audioBriefing.summary.needsAttention")}
            </div>
            <div className="mt-2 text-2xl font-semibold text-[var(--color-editorial-ink)]">{attentionCount}</div>
          </div>
          <div className="min-w-[180px] flex-1 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.audioBriefing.summary.configured")}
            </div>
            <div className="mt-2 text-2xl font-semibold text-[var(--color-editorial-ink)]">{configuredCount}/{totalCount}</div>
          </div>
        </div>

        {aivisModelsError ? <div className="rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-3 text-sm text-[#b45309]">{aivisModelsError}</div> : null}
        {xaiVoicesError ? <div className="rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-3 text-sm text-[#b45309]">{xaiVoicesError}</div> : null}
        {elevenLabsVoicesError ? <div className="rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-3 text-sm text-[#b45309]">{elevenLabsVoicesError}</div> : null}
        {geminiTTSVoicesError ? <div className="rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-3 text-sm text-[#b45309]">{geminiTTSVoicesError}</div> : null}

        {renderWarning(needsAivisAPIKey, t("settings.audioBriefing.aivisApiKeyWarningTitle"), t("settings.audioBriefing.aivisApiKeyWarningDetail"), "aivis")}
        {renderWarning(needsXAIAPIKey, t("settings.audioBriefing.xaiApiKeyWarningTitle"), t("settings.audioBriefing.xaiApiKeyWarningDetail"), "xai")}
        {renderWarning(needsFishAPIKey, t("settings.audioBriefing.fishApiKeyWarningTitle"), t("settings.audioBriefing.fishApiKeyWarningDetail"), "fish")}
        {renderWarning(needsElevenLabsAPIKey, t("settings.audioBriefing.elevenlabsApiKeyWarningTitle"), t("settings.audioBriefing.elevenlabsApiKeyWarningDetail"), "elevenlabs")}
        {renderWarning(needsOpenAIAPIKey, t("settings.audioBriefing.openAIApiKeyWarningTitle"), t("settings.audioBriefing.openAIApiKeyWarningDetail"), "openai")}
        {renderWarning(needsGeminiAccess, t("settings.audioBriefing.geminiAccessWarningTitle"), t("settings.audioBriefing.geminiAccessWarningDetail"))}

        <div className="space-y-3">
          {voiceSummaries.map(({ voice, status }) => (
            <AudioBriefingVoiceRow
              key={voice.persona}
              t={t}
              modelSelectLabels={modelSelectLabels}
              voice={voice}
              status={status}
              expanded={expandedPersonas.includes(voice.persona)}
              isDefaultPersona={voice.persona === defaultPersona}
              hasUserFishAPIKey={hasUserFishAPIKey}
              hasUserXAIAPIKey={hasUserXAIAPIKey}
              hasUserOpenAIAPIKey={hasUserOpenAIAPIKey}
              hasUserElevenLabsAPIKey={hasUserElevenLabsAPIKey}
              geminiTTSEnabled={geminiTTSEnabled}
              audioBriefingAivisModels={audioBriefingAivisModels}
              audioBriefingXAIVoices={audioBriefingXAIVoices}
              audioBriefingOpenAITTSVoices={audioBriefingOpenAITTSVoices}
              audioBriefingGeminiTTSVoices={audioBriefingGeminiTTSVoices}
              audioBriefingElevenLabsVoices={audioBriefingElevenLabsVoices}
              audioBriefingVoiceInputDrafts={audioBriefingVoiceInputDrafts}
              aivisModelsSyncing={aivisModelsSyncing}
              xaiVoicesSyncing={xaiVoicesSyncing}
              openAITTSVoicesSyncing={openAITTSVoicesSyncing}
              geminiTTSVoicesLoading={geminiTTSVoicesLoading}
              saving={saving}
              onTogglePersona={onTogglePersona}
              onUpdateVoice={onUpdateVoice}
              onOpenAivisPicker={onOpenAivisPicker}
              onOpenFishPicker={onOpenFishPicker}
              onOpenXAIPicker={onOpenXAIPicker}
              onOpenOpenAITTSPicker={onOpenOpenAITTSPicker}
              onOpenGeminiTTSPicker={onOpenGeminiTTSPicker}
              onOpenElevenLabsPicker={onOpenElevenLabsPicker}
              onUpdateVoiceNumberInput={onUpdateVoiceNumberInput}
              onResetVoiceNumberInput={onResetVoiceNumberInput}
              onSyncAivisModels={onSyncAivisModels}
              onSyncXAIVoices={onSyncXAIVoices}
              onSyncOpenAITTSVoices={onSyncOpenAITTSVoices}
              onLoadGeminiTTSVoices={onLoadGeminiTTSVoices}
              onPersistAudioBriefingVoices={onPersistAudioBriefingVoices}
              conversationMode={conversationMode}
            />
          ))}
        </div>
      </form>
    </SectionCard>
  );
}
