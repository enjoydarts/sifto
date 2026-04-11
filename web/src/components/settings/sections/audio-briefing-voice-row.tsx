"use client";

import { ChevronDown } from "lucide-react";
import ModelSelect from "@/components/settings/model-select";
import { AINavigatorAvatar } from "@/components/briefing/ai-navigator-avatar";
import ProviderVoiceSelectionCard from "@/components/settings/providers/provider-voice-selection-card";
import {
  buildElevenLabsTTSModelOptions,
  buildFishTTSModelOptions,
  buildGeminiTTSModelOptions,
  buildOpenAITTSModelOptions,
} from "@/components/settings/providers/tts-model-options";
import { formatTTSProviderLabel } from "@/components/settings/providers/tts-provider-metadata";
import {
  formatAivisVoiceStyleLabel,
  getAudioBriefingProviderCapabilities,
  resolveAivisVoiceSelection,
  resolveAzureSpeechVoiceSelection,
  resolveElevenLabsVoiceSelection,
  resolveGeminiTTSVoiceSelection,
  resolveOpenAITTSVoiceSelection,
  resolveXAIVoiceSelection,
  type VoiceStatus,
} from "@/components/settings/providers/tts-provider-readiness";
import { resolveTTSVoiceDisplay } from "@/components/settings/providers/tts-voice-display";
import type {
  AivisModelSnapshot,
  AudioBriefingPersonaVoice,
  AzureSpeechVoiceCatalogEntry,
  ElevenLabsVoiceCatalogEntry,
  GeminiTTSVoiceCatalogEntry,
  OpenAITTSVoiceSnapshot,
  XAIVoiceSnapshot,
} from "@/lib/api";
import type {
  AudioBriefingNumericInputField,
  AudioBriefingVoiceInputDrafts,
  ModelSelectLabels,
  Translate,
} from "@/components/settings/sections/audio-briefing-settings-types";

function formatAudioBriefingDecimalInput(value: number): string {
  if (!Number.isFinite(value)) return "";
  return value.toFixed(4).replace(/\.?0+$/, "");
}

export default function AudioBriefingVoiceRow({
  t,
  modelSelectLabels,
  voice,
  status,
  expanded,
  isDefaultPersona,
  hasUserFishAPIKey,
  hasUserXAIAPIKey,
  hasUserOpenAIAPIKey,
  hasUserElevenLabsAPIKey,
  hasUserAzureSpeechAPIKey,
  geminiTTSEnabled,
  audioBriefingAivisModels,
  audioBriefingXAIVoices,
  audioBriefingOpenAITTSVoices,
  audioBriefingGeminiTTSVoices,
  audioBriefingAzureSpeechVoices,
  audioBriefingElevenLabsVoices,
  audioBriefingVoiceInputDrafts,
  aivisModelsSyncing,
  xaiVoicesSyncing,
  openAITTSVoicesSyncing,
  geminiTTSVoicesLoading,
  azureSpeechVoicesLoading,
  saving,
  onTogglePersona,
  onUpdateVoice,
  onOpenAivisPicker,
  onOpenFishPicker,
  onOpenXAIPicker,
  onOpenOpenAITTSPicker,
  onOpenGeminiTTSPicker,
  onOpenAzureSpeechPicker,
  onOpenElevenLabsPicker,
  onUpdateVoiceNumberInput,
  onResetVoiceNumberInput,
  onSyncAivisModels,
  onSyncXAIVoices,
  onSyncOpenAITTSVoices,
  onLoadGeminiTTSVoices,
  onLoadAzureSpeechVoices,
  onPersistAudioBriefingVoices,
  conversationMode,
}: {
  t: Translate;
  modelSelectLabels: ModelSelectLabels;
  voice: AudioBriefingPersonaVoice;
  status: VoiceStatus;
  expanded: boolean;
  isDefaultPersona: boolean;
  hasUserFishAPIKey: boolean;
  hasUserXAIAPIKey: boolean;
  hasUserOpenAIAPIKey: boolean;
  hasUserElevenLabsAPIKey: boolean;
  hasUserAzureSpeechAPIKey: boolean;
  geminiTTSEnabled: boolean;
  audioBriefingAivisModels: AivisModelSnapshot[];
  audioBriefingXAIVoices: XAIVoiceSnapshot[];
  audioBriefingOpenAITTSVoices: OpenAITTSVoiceSnapshot[];
  audioBriefingGeminiTTSVoices: GeminiTTSVoiceCatalogEntry[];
  audioBriefingAzureSpeechVoices: AzureSpeechVoiceCatalogEntry[];
  audioBriefingElevenLabsVoices: ElevenLabsVoiceCatalogEntry[];
  audioBriefingVoiceInputDrafts: AudioBriefingVoiceInputDrafts;
  aivisModelsSyncing: boolean;
  xaiVoicesSyncing: boolean;
  openAITTSVoicesSyncing: boolean;
  geminiTTSVoicesLoading: boolean;
  azureSpeechVoicesLoading: boolean;
  saving: boolean;
  onTogglePersona: (persona: string) => void;
  onUpdateVoice: (persona: string, patch: Partial<AudioBriefingPersonaVoice>) => void;
  onOpenAivisPicker: (persona: string) => void;
  onOpenFishPicker: (persona: string) => void;
  onOpenXAIPicker: (persona: string) => void;
  onOpenOpenAITTSPicker: (persona: string) => void;
  onOpenGeminiTTSPicker: (persona: string) => void;
  onOpenAzureSpeechPicker: (persona: string) => void;
  onOpenElevenLabsPicker: (persona: string) => void;
  onUpdateVoiceNumberInput: (persona: string, field: AudioBriefingNumericInputField, raw: string) => void;
  onResetVoiceNumberInput: (persona: string, field: AudioBriefingNumericInputField) => void;
  onSyncAivisModels: () => void;
  onSyncXAIVoices: () => void;
  onSyncOpenAITTSVoices: () => void;
  onLoadGeminiTTSVoices: () => void;
  onLoadAzureSpeechVoices: () => void;
  onPersistAudioBriefingVoices: () => void;
  conversationMode: "single" | "duo";
}) {
  const providerCapabilities = getAudioBriefingProviderCapabilities(voice.tts_provider);
  const isAivisProvider = voice.tts_provider === "aivis";
  const isFishProvider = voice.tts_provider === "fish";
  const isXAIProvider = voice.tts_provider === "xai";
  const isOpenAIProvider = voice.tts_provider === "openai";
  const isGeminiProvider = voice.tts_provider === "gemini_tts";
  const isElevenLabsProvider = voice.tts_provider === "elevenlabs";
  const isAzureSpeechProvider = voice.tts_provider === "azure_speech";
  const aivisResolved = isAivisProvider ? resolveAivisVoiceSelection(audioBriefingAivisModels, voice) : null;
  const xaiResolved = isXAIProvider ? resolveXAIVoiceSelection(audioBriefingXAIVoices, voice) : null;
  const elevenLabsResolved = isElevenLabsProvider ? resolveElevenLabsVoiceSelection(audioBriefingElevenLabsVoices, voice) : null;
  const openAIResolved = isOpenAIProvider ? resolveOpenAITTSVoiceSelection(audioBriefingOpenAITTSVoices, voice) : null;
  const geminiResolved = isGeminiProvider ? resolveGeminiTTSVoiceSelection(audioBriefingGeminiTTSVoices, voice) : null;
  const azureSpeechResolved = isAzureSpeechProvider ? resolveAzureSpeechVoiceSelection(audioBriefingAzureSpeechVoices, voice) : null;
  const selectedVoiceDisplay = resolveTTSVoiceDisplay({
    provider: voice.tts_provider,
    voiceModel: voice.voice_model,
    voiceStyle: voice.voice_style,
    providerVoiceLabel: voice.provider_voice_label || "",
    providerVoiceDescription: voice.provider_voice_description || "",
    unsetText: t("settings.audioBriefing.unsetShort"),
    t,
    aivisResolved,
    xaiResolved,
    openAIResolved,
    geminiResolved,
    elevenLabsResolved,
    azureSpeechResolved,
  });
  const selectedVoiceLabel = selectedVoiceDisplay.label;
  const selectedVoiceDetail = selectedVoiceDisplay.detail;
  const toneClasses = status.tone === "ok"
    ? "border-[rgba(34,197,94,0.28)] bg-[rgba(240,253,244,0.72)]"
    : status.tone === "warn"
      ? "border-[rgba(245,158,11,0.35)] bg-[rgba(255,251,235,0.82)]"
      : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]";
  const badgeClasses = status.tone === "ok"
    ? "border-[rgba(34,197,94,0.24)] bg-[rgba(220,252,231,0.85)] text-[#166534]"
    : status.tone === "warn"
      ? "border-[rgba(245,158,11,0.24)] bg-[rgba(254,243,199,0.88)] text-[#b45309]"
      : "border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)]";

  return (
    <div className={`overflow-hidden rounded-[20px] border ${toneClasses}`}>
      <button
        type="button"
        onClick={() => onTogglePersona(voice.persona)}
        className="flex w-full flex-wrap items-center gap-3 px-4 py-4 text-left"
        aria-expanded={expanded}
      >
        <div className="flex min-w-[220px] flex-1 items-center gap-3">
          <div className="rounded-full border border-[var(--color-editorial-line)] bg-white p-1.5">
            <AINavigatorAvatar persona={voice.persona} className="size-10" />
          </div>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">
                {t(`settings.navigator.persona.${voice.persona}`, voice.persona)}
              </div>
              {isDefaultPersona ? (
                <span className="rounded-full border border-[var(--color-editorial-line)] bg-white px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-ink-soft)]">
                  {t("settings.audioBriefing.defaultPersonaBadge")}
                </span>
              ) : null}
            </div>
            <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">{voice.persona}</div>
          </div>
        </div>

        <div className="flex min-w-[180px] flex-1 flex-wrap items-center gap-2 text-[12px] text-[var(--color-editorial-ink-soft)]">
          <span className="rounded-full border border-[var(--color-editorial-line)] bg-white px-2.5 py-1">
            {formatTTSProviderLabel(voice.tts_provider, t)}
          </span>
          <span className="rounded-full border border-[var(--color-editorial-line)] bg-white px-2.5 py-1">
            {selectedVoiceLabel}
          </span>
          <span className="rounded-full border border-[var(--color-editorial-line)] bg-white px-2.5 py-1">
            {selectedVoiceDetail}
          </span>
        </div>

        <div className="ml-auto flex items-center gap-3">
          <div className={`rounded-full border px-3 py-1 text-[11px] font-semibold ${badgeClasses}`}>
            {status.label}
          </div>
          <ChevronDown
            aria-hidden="true"
            className={`size-4 text-[var(--color-editorial-ink-faint)] transition-transform ${expanded ? "rotate-180" : ""}`}
          />
        </div>
      </button>

      {expanded ? (
        <div className="border-t border-[var(--color-editorial-line)] bg-white/70 px-4 py-4">
          <div className="flex flex-wrap gap-3">
            <div className="min-w-[220px] flex-[1.4] rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.ttsProvider")}
              </div>
              <select
                value={voice.tts_provider}
                onChange={(e) => onUpdateVoice(voice.persona, { tts_provider: e.target.value })}
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              >
                {Array.from(new Set([
                  voice.tts_provider,
                  "aivis",
                  hasUserFishAPIKey || voice.tts_provider === "fish" ? "fish" : null,
                  hasUserXAIAPIKey || voice.tts_provider === "xai" ? "xai" : null,
                  hasUserOpenAIAPIKey || voice.tts_provider === "openai" ? "openai" : null,
                  geminiTTSEnabled || voice.tts_provider === "gemini_tts" ? "gemini_tts" : null,
                  "azure_speech",
                  "elevenlabs",
                  "mock",
                ].filter(Boolean) as string[])).map((provider) => (
                  <option key={`${voice.persona}-${provider}`} value={provider}>
                    {formatTTSProviderLabel(provider, t)}
                  </option>
                ))}
              </select>
              <p className="mt-3 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{status.detail}</p>
            </div>

            <div className="min-w-[260px] flex-[2]">
              <ProviderVoiceSelectionCard
                label={isElevenLabsProvider ? t("settings.audioBriefing.elevenlabsVoice") : t("settings.audioBriefing.voiceModel")}
                selectedLabel={selectedVoiceLabel}
                selectedDetail={selectedVoiceDetail}
                actionLabel={
                  providerCapabilities.supportsCatalogPicker
                    ? isAivisProvider
                      ? t("settings.audioBriefing.pickAivisVoice")
                      : isFishProvider
                        ? t("settings.audioBriefing.pickFishVoice")
                        : isXAIProvider
                          ? t("settings.audioBriefing.pickXaiVoice")
                          : isOpenAIProvider
                            ? t("settings.audioBriefing.pickOpenAITTSVoice")
                            : isGeminiProvider
                              ? t("settings.audioBriefing.pickGeminiTTSVoice")
                              : isAzureSpeechProvider
                                ? t("settings.audioBriefing.pickAzureSpeechVoice")
                              : isElevenLabsProvider
                                ? t("settings.audioBriefing.pickElevenLabsVoice")
                                : undefined
                    : undefined
                }
                onAction={
                  providerCapabilities.supportsCatalogPicker
                    ? isAivisProvider
                      ? () => onOpenAivisPicker(voice.persona)
                      : isFishProvider
                        ? () => onOpenFishPicker(voice.persona)
                        : isXAIProvider
                          ? () => onOpenXAIPicker(voice.persona)
                          : isOpenAIProvider
                            ? () => onOpenOpenAITTSPicker(voice.persona)
                              : isGeminiProvider
                                ? () => onOpenGeminiTTSPicker(voice.persona)
                                : isAzureSpeechProvider
                                  ? () => onOpenAzureSpeechPicker(voice.persona)
                              : isElevenLabsProvider
                                ? () => onOpenElevenLabsPicker(voice.persona)
                                : undefined
                    : undefined
                }
                actionDisabled={
                  isXAIProvider
                    ? !hasUserXAIAPIKey && !audioBriefingXAIVoices.length
                    : isOpenAIProvider
                      ? !hasUserOpenAIAPIKey && !audioBriefingOpenAITTSVoices.length
                        : isGeminiProvider
                          ? geminiTTSVoicesLoading && !audioBriefingGeminiTTSVoices.length
                        : isAzureSpeechProvider
                          ? !hasUserAzureSpeechAPIKey && !audioBriefingAzureSpeechVoices.length
                        : isElevenLabsProvider
                          ? !hasUserElevenLabsAPIKey && !audioBriefingElevenLabsVoices.length
                          : false
                }
              />
            </div>
          </div>

          {providerCapabilities.supportsSpeechTuning ? (
            <div className="mt-4 flex flex-wrap gap-3 text-[11px] text-[var(--color-editorial-ink-faint)]">
              <span>{`${t("settings.audioBriefing.voiceModel")}: ${voice.voice_model || "—"}`}</span>
              <span>{`${t("settings.audioBriefing.voiceStyle")}: ${formatAivisVoiceStyleLabel(voice.voice_style, t)}`}</span>
            </div>
          ) : isXAIProvider ? (
            <div className="mt-4 flex flex-wrap gap-3 text-[11px] text-[var(--color-editorial-ink-faint)]">
              <span>{`${t("settings.audioBriefing.voiceModel")}: ${voice.voice_model || "—"}`}</span>
              <span>{t("settings.audioBriefing.xaiVoiceStyleDisabled")}</span>
            </div>
          ) : isFishProvider ? (
            <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.85fr)]">
              <ModelSelect key={`audio-briefing-fish-tts-model-${voice.persona}-${voice.tts_provider}`} label={t("settings.audioBriefing.fishTTSModel")} value={voice.tts_model} onChange={(value) => onUpdateVoice(voice.persona, { tts_model: value })} options={buildFishTTSModelOptions(voice.tts_model)} labels={modelSelectLabels} variant="modal" />
              <ProviderVoiceSelectionCard
                label={t("settings.audioBriefing.fishVoice")}
                selectedLabel={voice.voice_model || t("settings.audioBriefing.fishVoiceEmpty")}
                selectedDetail={voice.voice_model || t("settings.audioBriefing.fishVoiceEmpty")}
              />
            </div>
          ) : isOpenAIProvider ? (
            <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.85fr)]">
              <ModelSelect key={`audio-briefing-openai-tts-model-${voice.persona}-${voice.tts_provider}`} label={t("settings.audioBriefing.openAITTSModel")} value={voice.tts_model} onChange={(value) => onUpdateVoice(voice.persona, { tts_model: value })} options={buildOpenAITTSModelOptions(voice.tts_model)} labels={modelSelectLabels} variant="modal" />
              <ProviderVoiceSelectionCard
                label={t("settings.audioBriefing.openAITTSVoice")}
                selectedLabel={openAIResolved?.name || t("settings.audioBriefing.openAITTSVoiceEmpty")}
                selectedDetail={openAIResolved?.description || voice.voice_model || t("settings.audioBriefing.openAITTSVoiceEmpty")}
              />
            </div>
          ) : isGeminiProvider ? (
            <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.85fr)]">
              <ModelSelect key={`audio-briefing-gemini-tts-model-${voice.persona}-${voice.tts_provider}`} label={t("settings.audioBriefing.geminiTTSModel")} value={voice.tts_model} onChange={(value) => onUpdateVoice(voice.persona, { tts_model: value })} options={buildGeminiTTSModelOptions(voice.tts_model)} labels={modelSelectLabels} variant="modal" />
              <ProviderVoiceSelectionCard
                label={t("settings.audioBriefing.geminiTTSVoice")}
                selectedLabel={geminiResolved?.label || t("settings.audioBriefing.geminiTTSVoiceEmpty")}
                selectedDetail={geminiResolved?.description || voice.voice_model || t("settings.audioBriefing.geminiTTSVoiceEmpty")}
              />
            </div>
          ) : isElevenLabsProvider ? (
            <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.85fr)]">
              <ModelSelect key={`audio-briefing-elevenlabs-tts-model-${voice.persona}-${voice.tts_provider}`} label={t("settings.audioBriefing.elevenlabsTTSModel")} value={voice.tts_model} onChange={(value) => onUpdateVoice(voice.persona, { tts_model: value })} options={buildElevenLabsTTSModelOptions(voice.tts_model, conversationMode)} labels={modelSelectLabels} variant="modal" />
              <ProviderVoiceSelectionCard
                label={t("settings.audioBriefing.elevenlabsVoice")}
                selectedLabel={selectedVoiceLabel || t("settings.audioBriefing.elevenlabsVoiceEmpty")}
                selectedDetail={selectedVoiceDetail || t("settings.audioBriefing.elevenlabsVoiceEmpty")}
              />
            </div>
          ) : isAzureSpeechProvider ? (
            <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.85fr)]">
              <ProviderVoiceSelectionCard
                label={t("settings.audioBriefing.azureSpeechVoice")}
                selectedLabel={selectedVoiceLabel || t("settings.audioBriefing.azureSpeechVoiceEmpty")}
                selectedDetail={selectedVoiceDetail || t("settings.audioBriefing.azureSpeechVoiceEmpty")}
                actionLabel={t("settings.audioBriefing.pickAzureSpeechVoice")}
                onAction={() => onOpenAzureSpeechPicker(voice.persona)}
                actionDisabled={azureSpeechVoicesLoading && !audioBriefingAzureSpeechVoices.length}
              />
              <ProviderVoiceSelectionCard
                label={t("settings.audioBriefing.azureSpeechCatalog")}
                selectedLabel={azureSpeechResolved?.locale || t("settings.audioBriefing.azureSpeechVoiceEmpty")}
                selectedDetail={azureSpeechResolved?.voice_id || voice.voice_model || t("settings.audioBriefing.azureSpeechVoiceEmpty")}
                actionLabel={t("settings.audioBriefing.refreshAzureSpeechCatalog")}
                onAction={onLoadAzureSpeechVoices}
                actionDisabled={azureSpeechVoicesLoading}
              />
            </div>
          ) : (
            <div className="mt-4 flex flex-wrap gap-3">
              <input value={voice.voice_model} onChange={(e) => onUpdateVoice(voice.persona, { voice_model: e.target.value })} placeholder={t("settings.audioBriefing.voiceModel")} className="min-w-[180px] flex-1 rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]" />
              {providerCapabilities.requiresVoiceStyle ? (
                <input value={voice.voice_style} onChange={(e) => onUpdateVoice(voice.persona, { voice_style: e.target.value })} placeholder={t("settings.audioBriefing.voiceStyle")} className="min-w-[180px] flex-1 rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]" />
              ) : null}
            </div>
          )}

          <div className="mt-3 flex flex-wrap gap-3">
            <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.speechRate")}</div>
              <input value={audioBriefingVoiceInputDrafts[voice.persona]?.speech_rate ?? formatAudioBriefingDecimalInput(voice.speech_rate)} onChange={(e) => onUpdateVoiceNumberInput(voice.persona, "speech_rate", e.target.value)} onBlur={() => onResetVoiceNumberInput(voice.persona, "speech_rate")} inputMode="decimal" className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]" />
            </label>

            {providerCapabilities.supportsSpeechTuning ? (
              <>
                <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.tempoDynamics")}</div>
                  <input value={audioBriefingVoiceInputDrafts[voice.persona]?.tempo_dynamics ?? formatAudioBriefingDecimalInput(voice.tempo_dynamics)} onChange={(e) => onUpdateVoiceNumberInput(voice.persona, "tempo_dynamics", e.target.value)} onBlur={() => onResetVoiceNumberInput(voice.persona, "tempo_dynamics")} inputMode="decimal" className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]" />
                </label>
                <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.emotionalIntensity")}</div>
                  <input value={audioBriefingVoiceInputDrafts[voice.persona]?.emotional_intensity ?? formatAudioBriefingDecimalInput(voice.emotional_intensity)} onChange={(e) => onUpdateVoiceNumberInput(voice.persona, "emotional_intensity", e.target.value)} onBlur={() => onResetVoiceNumberInput(voice.persona, "emotional_intensity")} inputMode="decimal" className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]" />
                </label>
                <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.lineBreakSilenceSeconds")}</div>
                  <input value={audioBriefingVoiceInputDrafts[voice.persona]?.line_break_silence_seconds ?? formatAudioBriefingDecimalInput(voice.line_break_silence_seconds)} onChange={(e) => onUpdateVoiceNumberInput(voice.persona, "line_break_silence_seconds", e.target.value)} onBlur={() => onResetVoiceNumberInput(voice.persona, "line_break_silence_seconds")} inputMode="decimal" className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]" />
                </label>
                <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.aivisVolume")}</div>
                  <input value={audioBriefingVoiceInputDrafts[voice.persona]?.aivis_volume ?? formatAudioBriefingDecimalInput(voice.volume_gain + 1)} onChange={(e) => onUpdateVoiceNumberInput(voice.persona, "aivis_volume", e.target.value)} onBlur={() => onResetVoiceNumberInput(voice.persona, "aivis_volume")} inputMode="decimal" className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]" />
                </label>
              </>
            ) : (
              <>
                <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.pitchAdjustment")}</div>
                  <input value={audioBriefingVoiceInputDrafts[voice.persona]?.pitch ?? formatAudioBriefingDecimalInput(voice.pitch)} onChange={(e) => onUpdateVoiceNumberInput(voice.persona, "pitch", e.target.value)} onBlur={() => onResetVoiceNumberInput(voice.persona, "pitch")} inputMode="decimal" className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]" />
                </label>
                <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.volumeAdjustment")}</div>
                  <input value={audioBriefingVoiceInputDrafts[voice.persona]?.volume_gain ?? formatAudioBriefingDecimalInput(voice.volume_gain)} onChange={(e) => onUpdateVoiceNumberInput(voice.persona, "volume_gain", e.target.value)} onBlur={() => onResetVoiceNumberInput(voice.persona, "volume_gain")} inputMode="decimal" className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]" />
                </label>
              </>
            )}
          </div>

          <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
            <p className="text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.inlineHelp")}</p>
            <div className="flex flex-wrap gap-2">
              {isAivisProvider ? (
                <button type="button" onClick={onSyncAivisModels} disabled={aivisModelsSyncing} className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60">
                  {aivisModelsSyncing ? t("aivisModels.syncing") : t("settings.audioBriefing.refreshCatalog")}
                </button>
              ) : isXAIProvider ? (
                <button type="button" onClick={onSyncXAIVoices} disabled={xaiVoicesSyncing} className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60">
                  {xaiVoicesSyncing ? t("settings.audioBriefing.syncingXaiCatalog") : t("settings.audioBriefing.refreshXaiCatalog")}
                </button>
              ) : isOpenAIProvider ? (
                <button type="button" onClick={onSyncOpenAITTSVoices} disabled={openAITTSVoicesSyncing} className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60">
                  {openAITTSVoicesSyncing ? t("settings.audioBriefing.syncingOpenAITTSCatalog") : t("settings.audioBriefing.refreshOpenAITTSCatalog")}
                </button>
              ) : isGeminiProvider ? (
                <button type="button" onClick={onLoadGeminiTTSVoices} disabled={geminiTTSVoicesLoading} className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60">
                  {geminiTTSVoicesLoading ? t("common.loading") : t("settings.audioBriefing.refreshGeminiTTSCatalog")}
                </button>
              ) : null}

              <button type="button" onClick={onPersistAudioBriefingVoices} disabled={saving} className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60">
                {saving ? t("common.saving") : t("settings.audioBriefing.savePersonaVoice")}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
