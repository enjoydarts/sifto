"use client";

import type { FormEvent } from "react";
import type { AivisUserDictionary } from "@/lib/api";
import { SectionCard } from "@/components/ui/section-card";
import ModelSelect, { type ModelOption } from "@/components/settings/model-select";
import ProviderVoiceSelectionCard from "@/components/settings/providers/provider-voice-selection-card";
import {
  buildElevenLabsTTSModelOptions,
  buildFishTTSModelOptions,
  buildGeminiTTSModelOptions,
  buildOpenAITTSModelOptions,
} from "@/components/settings/providers/tts-model-options";
import {
  formatAivisVoiceStyleLabel,
  type VoiceStatus,
} from "@/components/settings/providers/tts-provider-readiness";
import type { TTSProviderCapabilities } from "@/components/settings/providers/tts-provider-metadata";

type Translate = (key: string, fallback?: string) => string;

type ModelSelectLabels = {
  defaultOption: string;
  searchPlaceholder: string;
  noResults: string;
  providerAll: string;
  modalChoose: string;
  close: string;
  confirmTitle: string;
  confirmYes: string;
  confirmNo: string;
  confirmSuffix: string;
  providerColumn: string;
  modelColumn: string;
  pricingColumn: string;
};

type SummaryAudioVoiceInputDrafts = {
  speech_rate: string;
  tempo_dynamics: string;
  emotional_intensity: string;
  line_break_silence_seconds: string;
  pitch: string;
  volume_gain: string;
};

type SummaryAudioNumericInputField = keyof SummaryAudioVoiceInputDrafts;

export default function SummaryAudioSettingsSection({
  t,
  modelSelectLabels,
  form,
  state,
  actions,
  integrations,
}: {
  t: Translate;
  modelSelectLabels: ModelSelectLabels;
  form: {
    onSubmit: (event: FormEvent<HTMLFormElement>) => void;
    saving: boolean;
  };
  state: {
    voiceStatus: VoiceStatus;
    configured: boolean;
    provider: string;
    providerCapabilities: TTSProviderCapabilities;
    ttsModel: string;
    resolvedVoiceLabel: string;
    resolvedVoiceDetail: string;
    voicePickerDisabled: boolean;
    voiceStyle: string;
    voiceInputDrafts: SummaryAudioVoiceInputDrafts;
  };
  actions: {
    onChangeProvider: (provider: string) => void;
    onChangeTTSModel: (value: string) => void;
    onOpenVoicePicker: () => void;
    onChangeVoiceStyle: (value: string) => void;
    onChangeNumberInput: (field: SummaryAudioNumericInputField, value: string) => void;
    onBlurNumberInput: (field: SummaryAudioNumericInputField) => void;
    onOpenSystem: () => void;
    onChangeAivisUserDictionaryUUID: (value: string) => void;
  };
  integrations: {
    hasAivisAPIKey: boolean;
    hasFishAPIKey: boolean;
    aivisUserDictionaryUUID: string;
    aivisUserDictionariesLoading: boolean;
    aivisUserDictionaries: AivisUserDictionary[];
  };
}) {
  const { onSubmit, saving } = form;
  const {
    voiceStatus,
    configured,
    provider,
    providerCapabilities,
    ttsModel,
    resolvedVoiceLabel,
    resolvedVoiceDetail,
    voicePickerDisabled,
    voiceStyle,
    voiceInputDrafts,
  } = state;
  const {
    onChangeProvider,
    onChangeTTSModel,
    onOpenVoicePicker,
    onChangeVoiceStyle,
    onChangeNumberInput,
    onBlurNumberInput,
    onOpenSystem,
    onChangeAivisUserDictionaryUUID,
  } = actions;
  const {
    hasAivisAPIKey,
    hasFishAPIKey,
    aivisUserDictionaryUUID,
    aivisUserDictionariesLoading,
    aivisUserDictionaries,
  } = integrations;

  const providerToneClass =
    voiceStatus.tone === "ok"
      ? "border-[rgba(34,197,94,0.28)] bg-[rgba(240,253,244,0.72)]"
      : voiceStatus.tone === "warn"
        ? "border-[rgba(245,158,11,0.35)] bg-[rgba(255,251,235,0.82)]"
        : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]";

  const badgeToneClass =
    voiceStatus.tone === "ok"
      ? "border-[rgba(34,197,94,0.24)] bg-[rgba(220,252,231,0.85)] text-[#166534]"
      : voiceStatus.tone === "warn"
        ? "border-[rgba(245,158,11,0.24)] bg-[rgba(254,243,199,0.88)] text-[#b45309]"
        : "border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)]";

  const ttsModelOptions =
    provider === "fish"
      ? buildFishTTSModelOptions(ttsModel)
      : provider === "elevenlabs"
        ? buildElevenLabsTTSModelOptions(ttsModel)
        : provider === "openai"
          ? buildOpenAITTSModelOptions(ttsModel)
          : provider === "gemini_tts"
            ? buildGeminiTTSModelOptions(ttsModel)
            : [];

  const voiceLabel =
    provider === "elevenlabs"
      ? t("settings.summaryAudio.elevenlabsVoice")
      : t("settings.summaryAudio.voiceModel");

  const voiceHelp =
    provider === "elevenlabs"
      ? t("settings.summaryAudio.elevenlabsVoiceHelp")
      : t("settings.summaryAudio.voiceModelHelp");

  const voicePickerLabel =
    provider === "aivis"
      ? t("settings.audioBriefing.pickAivisVoice")
      : provider === "fish"
        ? t("settings.audioBriefing.pickFishVoice")
        : provider === "elevenlabs"
          ? t("settings.summaryAudio.pickElevenLabsVoice")
          : provider === "xai"
            ? t("settings.audioBriefing.pickXaiVoice")
            : provider === "openai"
              ? t("settings.audioBriefing.pickOpenAITTSVoice")
              : provider === "azure_speech"
                ? t("settings.summaryAudio.pickAzureSpeechVoice")
                : t("settings.audioBriefing.pickGeminiTTSVoice");

  const renderNumberField = (
    field: SummaryAudioNumericInputField,
    label: string,
  ) => (
    <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
        {label}
      </div>
      <input
        value={voiceInputDrafts[field]}
        onChange={(e) => onChangeNumberInput(field, e.target.value)}
        onBlur={() => onBlurNumberInput(field)}
        inputMode="decimal"
        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
      />
    </label>
  );

  return (
    <SectionCard>
      <form onSubmit={onSubmit} className="space-y-5">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.summaryAudio.summaryTitle")}</div>
            <p className="mt-1 max-w-3xl text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.summaryAudio.summaryHelp")}</p>
          </div>
          <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
            <button
              type="submit"
              disabled={saving}
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {saving ? t("common.saving") : t("settings.summaryAudio.saveSettings")}
            </button>
          </div>
        </div>

        <div className={`rounded-[20px] border px-4 py-4 ${providerToneClass}`}>
          <div className="flex flex-wrap items-center gap-3">
            <div className={`rounded-full border px-3 py-1 text-[11px] font-semibold ${badgeToneClass}`}>
              {voiceStatus.label}
            </div>
            <div className="text-sm text-[var(--color-editorial-ink-soft)]">{voiceStatus.detail}</div>
            <div className="ml-auto text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {configured ? t("settings.summaryAudio.playbackEnabled") : t("settings.summaryAudio.playbackDisabled")}
            </div>
          </div>
        </div>

        <div className="grid gap-3 lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)]">
          <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.summaryAudio.provider")}
            </div>
            <select
              value={provider}
              onChange={(e) => onChangeProvider(e.target.value)}
              className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
            >
              <option value="aivis">{t("settings.summaryAudio.provider.aivis")}</option>
              <option value="fish">{t("settings.summaryAudio.provider.fish")}</option>
              <option value="xai">{t("settings.summaryAudio.provider.xai")}</option>
              <option value="openai">{t("settings.summaryAudio.provider.openai")}</option>
              <option value="gemini_tts">{t("settings.summaryAudio.provider.gemini_tts")}</option>
              <option value="elevenlabs">{t("settings.summaryAudio.provider.elevenlabs")}</option>
              <option value="azure_speech">{t("settings.summaryAudio.provider.azure_speech")}</option>
            </select>
          </label>

          {providerCapabilities.supportsSeparateTTSModel ? (
            <ModelSelect
              key={`summary-audio-tts-model-${provider}`}
              label={t("settings.summaryAudio.ttsModel")}
              value={ttsModel}
              onChange={onChangeTTSModel}
              options={ttsModelOptions}
              labels={modelSelectLabels}
              variant="modal"
            />
          ) : (
            <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {voiceLabel}
              </div>
              <div className="mt-2 text-sm text-[var(--color-editorial-ink-soft)]">{voiceHelp}</div>
            </div>
          )}
        </div>

        <ProviderVoiceSelectionCard
          label={voiceLabel}
          selectedLabel={resolvedVoiceLabel}
          selectedDetail={resolvedVoiceDetail}
          actionLabel={voicePickerLabel}
          onAction={onOpenVoicePicker}
          actionDisabled={voicePickerDisabled}
        />

        {providerCapabilities.requiresVoiceStyle ? (
          <label className="block rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.summaryAudio.voiceStyle")}
            </div>
            {provider === "aivis" ? (
              <div className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]">
                {formatAivisVoiceStyleLabel(voiceStyle, t)}
              </div>
            ) : (
              <input
                value={voiceStyle}
                onChange={(e) => onChangeVoiceStyle(e.target.value)}
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              />
            )}
            <p className="mt-2 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
              {t("settings.summaryAudio.voiceStyleHelp")}
            </p>
          </label>
        ) : null}

        <div className="grid gap-3 lg:grid-cols-3">
          {renderNumberField("speech_rate", t("settings.summaryAudio.speechRate"))}
          {renderNumberField("emotional_intensity", t("settings.summaryAudio.emotionalIntensity"))}
          {renderNumberField("tempo_dynamics", t("settings.summaryAudio.tempoDynamics"))}
        </div>

        <div className="grid gap-3 lg:grid-cols-3">
          {renderNumberField("line_break_silence_seconds", t("settings.summaryAudio.lineBreakSilenceSeconds"))}
          {renderNumberField("pitch", t("settings.summaryAudio.pitch"))}
          {renderNumberField("volume_gain", t("settings.summaryAudio.volumeGain"))}
        </div>

        {provider === "aivis" ? (
          !hasAivisAPIKey ? (
            <div className="flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
              <div>
                <div className="font-semibold">{t("settings.summaryAudio.aivisApiKeyWarningTitle")}</div>
                <div className="mt-1 leading-6">{t("settings.summaryAudio.aivisApiKeyWarningDetail")}</div>
              </div>
              <button
                type="button"
                onClick={onOpenSystem}
                className="inline-flex min-h-10 items-center justify-center rounded-full border border-[rgba(180,83,9,0.22)] bg-white px-4 py-2 text-sm font-medium text-[#92400e] hover:bg-[rgba(255,255,255,0.72)]"
              >
                {t("settings.summaryAudio.openApiKeys")}
              </button>
            </div>
          ) : (
            <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,0.8fr)]">
              <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                  {t("settings.summaryAudio.aivisDictionary")}
                </div>
                <select
                  value={aivisUserDictionaryUUID}
                  onChange={(e) => onChangeAivisUserDictionaryUUID(e.target.value)}
                  disabled={aivisUserDictionariesLoading || aivisUserDictionaries.length === 0}
                  className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)] disabled:opacity-60"
                >
                  <option value="">{t("settings.aivisDictionaryUnset")}</option>
                  {aivisUserDictionaries.map((item) => (
                    <option key={item.uuid} value={item.uuid}>
                      {`${item.name} (${item.word_count})`}
                    </option>
                  ))}
                </select>
                <p className="mt-2 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {t("settings.summaryAudio.aivisDictionaryHelp")}
                </p>
              </label>
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">
                <div className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.summaryAudio.aivisDictionaryTitle")}</div>
                <p className="mt-2">{t("settings.summaryAudio.aivisDictionaryDetail")}</p>
              </div>
            </div>
          )
        ) : provider === "fish" ? (
          !hasFishAPIKey ? (
            <div className="flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
              <div>
                <div className="font-semibold">{t("settings.summaryAudio.fishApiKeyWarningTitle")}</div>
                <div className="mt-1 leading-6">{t("settings.summaryAudio.fishApiKeyWarningDetail")}</div>
              </div>
              <button
                type="button"
                onClick={onOpenSystem}
                className="inline-flex min-h-10 items-center justify-center rounded-full border border-[rgba(180,83,9,0.22)] bg-white px-4 py-2 text-sm font-medium text-[#92400e] hover:bg-[rgba(255,255,255,0.72)]"
              >
                {t("settings.summaryAudio.openApiKeys")}
              </button>
            </div>
          ) : (
            <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">
              <div className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.summaryAudio.fishVoiceTitle")}</div>
              <p className="mt-2">{t("settings.summaryAudio.fishVoiceDetail")}</p>
            </div>
          )
        ) : null}
      </form>
    </SectionCard>
  );
}
