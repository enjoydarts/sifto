"use client";

import { useMemo } from "react";
import { X } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import {
  formatAudioBriefingPresetVoiceDetail,
  formatAudioBriefingPresetVoiceLabel,
} from "@/components/settings/audio-briefing-preset-modal-helpers";
import type { AudioBriefingPersonaVoice } from "@/lib/api";

export type AudioBriefingPresetSaveModalProps = {
  open: boolean;
  saving: boolean;
  presetName: string;
  defaultPersonaMode: "fixed" | "random";
  defaultPersona: string;
  conversationMode: "single" | "duo";
  voices: AudioBriefingPersonaVoice[];
  onClose: () => void;
  onChangeName: (value: string) => void;
  onSave: () => void;
};

export default function AudioBriefingPresetSaveModal({
  open,
  saving,
  presetName,
  defaultPersonaMode,
  defaultPersona,
  conversationMode,
  voices,
  onClose,
  onChangeName,
  onSave,
}: AudioBriefingPresetSaveModalProps) {
  const { t } = useI18n();

  const configuredCount = useMemo(
    () => voices.filter((voice) => voice.voice_model.trim().length > 0).length,
    [voices],
  );

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-3xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.presetSaveTitle")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.presetSaveSubtitle")}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            aria-label={t("common.close")}
          >
            <X className="size-4" />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
          <div className="space-y-5">
            <label className="block">
              <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.presetName")}</div>
              <input
                value={presetName}
                onChange={(e) => onChangeName(e.target.value)}
                placeholder={t("settings.audioBriefing.presetNamePlaceholder")}
                className="mt-2 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-white px-4 py-3 text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
              />
              <p className="mt-2 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.presetNameHelp")}</p>
            </label>

            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                  {t("settings.audioBriefing.defaultPersona")}
                </div>
                <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                  {defaultPersonaMode === "random"
                    ? t("settings.personaMode.random")
                    : t(`settings.navigator.persona.${defaultPersona}`, defaultPersona)}
                </div>
              </div>
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                  {t("settings.audioBriefing.conversationMode")}
                </div>
                <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                  {t(`settings.audioBriefing.conversationMode.${conversationMode}`)}
                </div>
              </div>
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                  {t("settings.audioBriefing.summary.configured")}
                </div>
                <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                  {`${configuredCount}/${voices.length}`}
                </div>
              </div>
            </div>

            <div className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
              <div className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.presetSaveIncludes")}
              </div>
              <div className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.presetSaveIncludesHelp")}</div>
              <div className="mt-4 grid gap-2 sm:grid-cols-2">
                {voices.map((voice) => (
                  <div key={voice.persona} className="rounded-[16px] border border-[var(--color-editorial-line)] bg-white px-4 py-3">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)]">
                      {t(`settings.navigator.persona.${voice.persona}`, voice.persona)}
                    </div>
                    <div className="mt-1 text-sm font-medium text-[var(--color-editorial-ink)]">
                      {formatAudioBriefingPresetVoiceLabel(voice, t)}
                    </div>
                    <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">
                      {formatAudioBriefingPresetVoiceDetail(voice, t)}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>

        <div className="shrink-0 flex flex-wrap items-center justify-end gap-2 border-t border-[var(--color-editorial-line)] px-5 py-4">
          <button
            type="button"
            onClick={onClose}
            className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
          >
            {t("common.cancel")}
          </button>
          <button
            type="button"
            onClick={onSave}
            disabled={saving || !presetName.trim()}
            className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {saving ? t("common.saving") : t("settings.audioBriefing.presetSaveConfirm")}
          </button>
        </div>
      </div>
    </div>
  );
}
