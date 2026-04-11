"use client";

import type { FormEvent } from "react";
import Link from "next/link";
import ModelSelect, { type ModelOption } from "@/components/settings/model-select";
import type { ModelSelectLabels, Translate } from "@/components/settings/sections/audio-briefing-settings-types";
import { SectionCard } from "@/components/ui/section-card";
import type { AivisUserDictionary } from "@/lib/api";

type TranslateWithVars = (
  t: Translate,
  key: string,
  vars: Record<string, string | number>,
  fallback?: string,
) => string;

type AudioBriefingScheduleSelection = "interval3h" | "interval6h" | "fixed3x";

type NavigatorPersonaCard = {
  key: string;
};

export default function AudioBriefingSettingsSection({
  t,
  tWithVars,
  modelSelectLabels,
  settingsForm,
  settingsState,
  duoReadiness,
  scriptModels,
  dictionaryState,
  actions,
}: {
  t: Translate;
  tWithVars: TranslateWithVars;
  modelSelectLabels: ModelSelectLabels;
  settingsForm: {
    onSubmitSettings: (event: FormEvent<HTMLFormElement>) => void;
    savingSettings: boolean;
    onSubmitModels: (event: FormEvent<HTMLFormElement>) => void;
    savingModels: boolean;
  };
  settingsState: {
    presetsLoading: boolean;
    presetsCount: number;
    enabled: boolean;
    programName: string;
    scheduleSelection: AudioBriefingScheduleSelection;
    articlesPerEpisode: string;
    targetDurationMinutes: string;
    chunkTrailingSilenceSeconds: string;
    conversationMode: "single" | "duo";
    defaultPersonaMode: "fixed" | "random";
    defaultPersona: string;
    navigatorPersonaCards: NavigatorPersonaCard[];
    bgmEnabled: boolean;
    bgmPrefix: string;
  };
  duoReadiness: {
    geminiDuoReady: boolean;
    geminiDuoCompatiblePersonaCount: number;
    geminiDuoCompatibleModel: string;
    fishDuoReady: boolean;
    fishDuoDistinctVoiceCount: number;
    elevenLabsDuoReady: boolean;
    elevenLabsDuoDistinctVoiceCount: number;
  };
  scriptModels: {
    audioBriefingScriptModel: string;
    audioBriefingScriptOptions: ModelOption[];
    audioBriefingScriptFallbackModel: string;
    audioBriefingScriptFallbackOptions: ModelOption[];
  };
  dictionaryState: {
    hasAivisAPIKey: boolean;
    aivisUserDictionariesLoading: boolean;
    aivisUserDictionariesError: string | null;
    aivisUserDictionaries: AivisUserDictionary[];
    aivisUserDictionaryUUID: string;
    savingAivisDictionary: boolean;
    deletingAivisDictionary: boolean;
    savedAivisUserDictionaryUUID: string;
  };
  actions: {
    onOpenPresetApplyModal: () => void;
    onOpenPresetSaveModal: () => void;
    onChangeEnabled: (value: boolean) => void;
    onChangeProgramName: (value: string) => void;
    onChangeScheduleSelection: (value: AudioBriefingScheduleSelection) => void;
    onChangeArticlesPerEpisode: (value: string) => void;
    onChangeTargetDurationMinutes: (value: string) => void;
    onChangeChunkTrailingSilenceSeconds: (value: string) => void;
    onChangeConversationMode: (value: "single" | "duo") => void;
    onChangeDefaultPersonaMode: (value: "fixed" | "random") => void;
    onChangeDefaultPersona: (value: string) => void;
    onChangeBGMEnabled: (value: boolean) => void;
    onChangeBGMPrefix: (value: string) => void;
    onChangeAudioBriefingScriptModel: (value: string) => void;
    onChangeAudioBriefingScriptFallbackModel: (value: string) => void;
    onRefreshAivisUserDictionaries: () => void;
    onChangeAivisUserDictionaryUUID: (value: string) => void;
    onSaveAivisUserDictionary: () => void;
    onClearAivisUserDictionary: () => void;
    onOpenSystem: () => void;
    onOpenSystemForProvider: (provider: string) => void;
  };
}) {
  const {
    onSubmitSettings,
    savingSettings,
    onSubmitModels,
    savingModels,
  } = settingsForm;
  const {
    presetsLoading,
    presetsCount,
    enabled,
    programName,
    scheduleSelection,
    articlesPerEpisode,
    targetDurationMinutes,
    chunkTrailingSilenceSeconds,
    conversationMode,
    defaultPersonaMode,
    defaultPersona,
    navigatorPersonaCards,
    bgmEnabled,
    bgmPrefix,
  } = settingsState;
  const {
    geminiDuoReady,
    geminiDuoCompatiblePersonaCount,
    geminiDuoCompatibleModel,
    fishDuoReady,
    fishDuoDistinctVoiceCount,
    elevenLabsDuoReady,
    elevenLabsDuoDistinctVoiceCount,
  } = duoReadiness;
  const {
    audioBriefingScriptModel,
    audioBriefingScriptOptions,
    audioBriefingScriptFallbackModel,
    audioBriefingScriptFallbackOptions,
  } = scriptModels;
  const {
    hasAivisAPIKey,
    aivisUserDictionariesLoading,
    aivisUserDictionariesError,
    aivisUserDictionaries,
    aivisUserDictionaryUUID,
    savingAivisDictionary,
    deletingAivisDictionary,
    savedAivisUserDictionaryUUID,
  } = dictionaryState;
  const {
    onOpenPresetApplyModal,
    onOpenPresetSaveModal,
    onChangeEnabled,
    onChangeProgramName,
    onChangeScheduleSelection,
    onChangeArticlesPerEpisode,
    onChangeTargetDurationMinutes,
    onChangeChunkTrailingSilenceSeconds,
    onChangeConversationMode,
    onChangeDefaultPersonaMode,
    onChangeDefaultPersona,
    onChangeBGMEnabled,
    onChangeBGMPrefix,
    onChangeAudioBriefingScriptModel,
    onChangeAudioBriefingScriptFallbackModel,
    onRefreshAivisUserDictionaries,
    onChangeAivisUserDictionaryUUID,
    onSaveAivisUserDictionary,
    onClearAivisUserDictionary,
    onOpenSystem,
    onOpenSystemForProvider,
  } = actions;
  return (
    <>
      <SectionCard>
        <form onSubmit={onSubmitSettings} className="space-y-5">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.summaryTitle")}</div>
              <p className="mt-1 max-w-3xl text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.summaryHelp")}</p>
            </div>
            <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
              <button
                type="button"
                onClick={onOpenPresetApplyModal}
                disabled={presetsLoading && presetsCount === 0}
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              >
                {presetsLoading && presetsCount === 0 ? t("common.loading") : t("settings.audioBriefing.applyPreset")}
              </button>
              <button
                type="button"
                onClick={onOpenPresetSaveModal}
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-60"
              >
                {t("settings.audioBriefing.savePreset")}
              </button>
              <button
                type="submit"
                disabled={savingSettings}
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {savingSettings ? t("common.saving") : t("settings.audioBriefing.saveSettings")}
              </button>
              <Link
                href="/audio-briefings"
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              >
                {t("settings.audioBriefing.openEpisodes")}
              </Link>
            </div>
          </div>

          <div className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
            <div className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.modeGuideTitle", "Single / Duo guide")}</div>
            <p className="mt-2">{t("settings.audioBriefing.modeGuideBody", "Single keeps the current one-person narration path. Duo adds a host-and-partner conversation, which increases turns, processing time, and TTS cost, but makes the listening experience more conversational.")}</p>
            <p className="mt-2">
              {conversationMode === "duo"
                ? t("settings.audioBriefing.modeGuideDuoActive", "Duo is currently selected. If persona mode is random, the host follows the same random selection as single mode and the partner is chosen from a different persona.")
                : t("settings.audioBriefing.modeGuideSingleActive", "Single is currently selected. This is the existing stable path, and you can switch back to it at any time if duo quality is not where you want it yet.")}
            </p>
          </div>

          <div className="flex flex-wrap items-stretch gap-3">
            <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.enableTitle")}
              </div>
              <div className="mt-3 flex items-center justify-between gap-3">
                <div className="text-sm font-medium text-[var(--color-editorial-ink)] whitespace-nowrap">
                  {enabled ? t("settings.on") : t("settings.off")}
                </div>
                <input
                  type="checkbox"
                  checked={enabled}
                  onChange={(e) => onChangeEnabled(e.target.checked)}
                  className="size-4 rounded border-[var(--color-editorial-line-strong)]"
                />
              </div>
            </label>

            <label className="flex min-w-[260px] flex-[1.5] flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.programName")}
              </div>
              <input
                type="text"
                value={programName}
                onChange={(e) => onChangeProgramName(e.target.value)}
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              />
              <p className="mt-2 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.programNameHelp")}
              </p>
            </label>

            <label className="flex min-w-[240px] flex-[1.15] flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.schedule")}
              </div>
              <select
                value={scheduleSelection}
                onChange={(e) => onChangeScheduleSelection(e.target.value as AudioBriefingScheduleSelection)}
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              >
                <option value="interval3h">{t("settings.audioBriefing.interval3h")}</option>
                <option value="interval6h">{t("settings.audioBriefing.interval6h")}</option>
                <option value="fixed3x">{t("settings.audioBriefing.fixed3x")}</option>
              </select>
              <p className="mt-2 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.scheduleHelp")}
              </p>
            </label>

            <label className="flex min-w-[180px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.articlesPerEpisode")}
              </div>
              <input
                value={articlesPerEpisode}
                onChange={(e) => onChangeArticlesPerEpisode(e.target.value)}
                inputMode="numeric"
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              />
            </label>

            <label className="flex min-w-[180px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.targetDuration")}
              </div>
              <input
                value={targetDurationMinutes}
                onChange={(e) => onChangeTargetDurationMinutes(e.target.value)}
                inputMode="numeric"
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              />
            </label>

            <label className="flex min-w-[180px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.chunkTrailingSilenceSeconds")}
              </div>
              <input
                value={chunkTrailingSilenceSeconds}
                onChange={(e) => onChangeChunkTrailingSilenceSeconds(e.target.value)}
                inputMode="decimal"
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              />
              <p className="mt-2 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.chunkTrailingSilenceSecondsHelp")}
              </p>
            </label>

            <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.conversationMode")}
              </div>
              <select
                value={conversationMode}
                onChange={(e) => onChangeConversationMode(e.target.value === "duo" ? "duo" : "single")}
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              >
                <option value="single">{t("settings.audioBriefing.conversationMode.single")}</option>
                <option value="duo">{t("settings.audioBriefing.conversationMode.duo")}</option>
              </select>
            </label>

            <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.personaMode.label")}
              </div>
              <select
                value={defaultPersonaMode}
                onChange={(e) => onChangeDefaultPersonaMode(e.target.value === "random" ? "random" : "fixed")}
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              >
                <option value="fixed">{t("settings.personaMode.fixed")}</option>
                <option value="random">{t("settings.personaMode.random")}</option>
              </select>
            </label>

            <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.defaultPersona")}
              </div>
              <select
                value={defaultPersona}
                onChange={(e) => onChangeDefaultPersona(e.target.value)}
                disabled={defaultPersonaMode === "random"}
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              >
                {navigatorPersonaCards.map((persona) => (
                  <option key={persona.key} value={persona.key}>
                    {t(`settings.navigator.persona.${persona.key}`, persona.key)}
                  </option>
                ))}
              </select>
              <p className="mt-2 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)]">
                {defaultPersonaMode === "random"
                  ? t("settings.audioBriefing.randomPersonaHelp")
                  : t("settings.audioBriefing.defaultPersonaHelp")}
              </p>
            </label>

            <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.bgmTitle")}
              </div>
              <div className="mt-3 flex items-center justify-between gap-3">
                <div className="whitespace-nowrap text-sm font-medium text-[var(--color-editorial-ink)]">
                  {bgmEnabled ? t("settings.on") : t("settings.off")}
                </div>
                <input
                  type="checkbox"
                  checked={bgmEnabled}
                  onChange={(e) => onChangeBGMEnabled(e.target.checked)}
                  className="size-4 rounded border-[var(--color-editorial-line-strong)]"
                />
              </div>
              <p className="mt-2 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.bgmHelp")}
              </p>
            </label>

            <label className="flex min-w-[260px] flex-[1.4] flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.bgmPrefix")}
              </div>
              <input
                value={bgmPrefix}
                onChange={(e) => onChangeBGMPrefix(e.target.value)}
                placeholder="audio-briefings/bgm/"
                className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
              />
              <p className="mt-2 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.bgmPrefixHelp")}
              </p>
            </label>
          </div>

          {conversationMode === "duo" ? (
            <div className="grid gap-3 lg:grid-cols-2">
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] px-4 py-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                  {t("settings.audioBriefing.duoHostRuleTitle", "Host selection")}
                </div>
                <p className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">
                  {defaultPersonaMode === "random"
                    ? t("settings.audioBriefing.duoHostRuleRandom", "Because persona mode is random, the host also follows the same random selection used by single mode.")
                    : t("settings.audioBriefing.duoHostRuleFixed", "Because persona mode is fixed, the selected default persona will always act as the host.")}
                </p>
              </div>
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] px-4 py-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                  {t("settings.audioBriefing.duoPartnerRuleTitle", "Partner selection")}
                </div>
                <p className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">
                  {t("settings.audioBriefing.duoPartnerRuleBody", "The partner is picked from a different persona than the host. Make sure multiple persona voices are configured if you plan to use duo regularly.")}
                </p>
              </div>
              <div
                className={`rounded-[18px] border px-4 py-4 ${
                  geminiDuoReady
                    ? "border-[rgba(34,197,94,0.24)] bg-[rgba(240,253,244,0.82)]"
                    : "border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)]"
                }`}
              >
                <div
                  className={`text-[11px] font-semibold uppercase tracking-[0.14em] ${
                    geminiDuoReady ? "text-[#166534]" : "text-[#b45309]"
                  }`}
                >
                  {geminiDuoReady
                    ? t("settings.audioBriefing.geminiDuoReadyTitle")
                    : t("settings.audioBriefing.geminiDuoNeedsSetupTitle")}
                </div>
                <p className={`mt-2 text-sm leading-6 ${geminiDuoReady ? "text-[#166534]" : "text-[#b45309]"}`}>
                  {geminiDuoReady
                    ? tWithVars(t, "settings.audioBriefing.geminiDuoReadyDetail", {
                        count: geminiDuoCompatiblePersonaCount,
                        model: geminiDuoCompatibleModel,
                      })
                    : t("settings.audioBriefing.geminiDuoNeedsSetupDetail")}
                </p>
              </div>
              <div
                className={`rounded-[18px] border px-4 py-4 ${
                  fishDuoReady
                    ? "border-[rgba(34,197,94,0.24)] bg-[rgba(240,253,244,0.82)]"
                    : "border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)]"
                }`}
              >
                <div
                  className={`text-[11px] font-semibold uppercase tracking-[0.14em] ${
                    fishDuoReady ? "text-[#166534]" : "text-[#b45309]"
                  }`}
                >
                  {fishDuoReady
                    ? t("settings.audioBriefing.fishDuoReadyTitle")
                    : t("settings.audioBriefing.fishDuoNeedsSetupTitle")}
                </div>
                <p className={`mt-2 text-sm leading-6 ${fishDuoReady ? "text-[#166534]" : "text-[#b45309]"}`}>
                  {fishDuoReady
                    ? tWithVars(t, "settings.audioBriefing.fishDuoReadyDetail", {
                        count: fishDuoDistinctVoiceCount,
                        model: "s2-pro",
                      })
                    : t("settings.audioBriefing.fishDuoNeedsSetupDetail")}
                </p>
              </div>
              <div
                className={`rounded-[18px] border px-4 py-4 ${
                  elevenLabsDuoReady
                    ? "border-[rgba(34,197,94,0.24)] bg-[rgba(240,253,244,0.82)]"
                    : "border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)]"
                }`}
              >
                <div
                  className={`text-[11px] font-semibold uppercase tracking-[0.14em] ${
                    elevenLabsDuoReady ? "text-[#166534]" : "text-[#b45309]"
                  }`}
                >
                  {elevenLabsDuoReady
                    ? t("settings.audioBriefing.elevenlabsDuoReadyTitle")
                    : t("settings.audioBriefing.elevenlabsDuoNeedsSetupTitle")}
                </div>
                <p className={`mt-2 text-sm leading-6 ${elevenLabsDuoReady ? "text-[#166534]" : "text-[#b45309]"}`}>
                  {elevenLabsDuoReady
                    ? tWithVars(t, "settings.audioBriefing.elevenlabsDuoReadyDetail", {
                        count: elevenLabsDuoDistinctVoiceCount,
                        model: "eleven_v3",
                      })
                    : t("settings.audioBriefing.elevenlabsDuoNeedsSetupDetail")}
                </p>
              </div>
            </div>
          ) : null}
        </form>
      </SectionCard>

      <SectionCard>
        <form onSubmit={onSubmitModels} className="space-y-4">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.model.audioBriefingScript")}</div>
              <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.scriptModelHelp")}
              </p>
            </div>
            <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
              <button
                type="submit"
                disabled={savingModels}
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
              >
                {savingModels ? t("common.saving") : t("settings.saveModels")}
              </button>
            </div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <ModelSelect
              label={t("settings.model.audioBriefingScript")}
              value={audioBriefingScriptModel}
              onChange={onChangeAudioBriefingScriptModel}
              options={audioBriefingScriptOptions}
              labels={modelSelectLabels}
              variant="modal"
            />
            <ModelSelect
              label={t("settings.model.audioBriefingScriptFallback")}
              value={audioBriefingScriptFallbackModel}
              onChange={onChangeAudioBriefingScriptFallbackModel}
              options={audioBriefingScriptFallbackOptions}
              labels={modelSelectLabels}
              variant="modal"
            />
          </div>
        </form>
      </SectionCard>

      <SectionCard>
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.aivisDictionaryTitle")}</div>
            <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.aivisDictionaryDescription")}</p>
          </div>
          <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
            <button
              type="button"
              onClick={onRefreshAivisUserDictionaries}
              disabled={!hasAivisAPIKey || aivisUserDictionariesLoading}
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
            >
              {aivisUserDictionariesLoading ? t("common.loading") : t("common.refresh")}
            </button>
          </div>
        </div>

        {!hasAivisAPIKey ? (
          <div className="mt-4 flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
            <div>
              <div className="font-semibold">{t("settings.audioBriefing.aivisApiKeyWarningTitle")}</div>
              <div className="mt-1 leading-6">{t("settings.aivisDictionaryRequiresApiKey")}</div>
            </div>
            <button
              type="button"
              onClick={() => onOpenSystemForProvider("aivis")}
              className="inline-flex min-h-10 items-center justify-center rounded-full border border-[rgba(180,83,9,0.22)] bg-white px-4 py-2 text-sm font-medium text-[#92400e] hover:bg-[rgba(255,255,255,0.72)]"
            >
              {t("settings.audioBriefing.openApiKeys")}
            </button>
          </div>
        ) : (
          <div className="mt-4 space-y-4">
            <div className="space-y-2">
              <label className="text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.aivisDictionarySelectLabel")}
              </label>
              <select
                value={aivisUserDictionaryUUID}
                onChange={(e) => onChangeAivisUserDictionaryUUID(e.target.value)}
                disabled={aivisUserDictionariesLoading || aivisUserDictionaries.length === 0}
                className="w-full rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3 text-sm text-[var(--color-editorial-ink)] disabled:opacity-60"
              >
                <option value="">{t("settings.aivisDictionaryUnset")}</option>
                {aivisUserDictionaries.map((item) => (
                  <option key={item.uuid} value={item.uuid}>
                    {`${item.name} (${item.word_count})`}
                  </option>
                ))}
              </select>
              {aivisUserDictionariesError ? (
                <p className="text-xs text-[var(--color-editorial-danger)]">{aivisUserDictionariesError}</p>
              ) : null}
              {!aivisUserDictionariesLoading && aivisUserDictionaries.length === 0 ? (
                <p className="text-xs text-[var(--color-editorial-ink-faint)]">{t("settings.aivisDictionaryEmpty")}</p>
              ) : null}
              {aivisUserDictionaryUUID ? (
                <p className="text-xs text-[var(--color-editorial-ink-faint)]">
                  {aivisUserDictionaries.find((item) => item.uuid === aivisUserDictionaryUUID)?.description || t("settings.aivisDictionarySelected")}
                </p>
              ) : null}
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={onSaveAivisUserDictionary}
                disabled={!aivisUserDictionaryUUID || savingAivisDictionary}
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
              >
                {savingAivisDictionary ? t("common.saving") : t("common.save")}
              </button>
              <button
                type="button"
                onClick={onClearAivisUserDictionary}
                disabled={!savedAivisUserDictionaryUUID || deletingAivisDictionary}
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink)] disabled:opacity-60"
              >
                {deletingAivisDictionary ? t("common.loading") : t("settings.delete")}
              </button>
              <button
                type="button"
                onClick={onOpenSystem}
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)]"
              >
                {t("settings.audioBriefing.openApiKeys")}
              </button>
            </div>
          </div>
        )}
      </SectionCard>
    </>
  );
}
