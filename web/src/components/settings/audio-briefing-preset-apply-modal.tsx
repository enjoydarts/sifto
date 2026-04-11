"use client";

import { useEffect, useMemo, useState } from "react";
import { Search, X } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import {
  formatAudioBriefingPresetVoiceDetail,
  formatAudioBriefingPresetVoiceLabel,
  normalizeAudioBriefingPresetVoices,
} from "@/components/settings/audio-briefing-preset-modal-helpers";
import type { AudioBriefingPreset } from "@/lib/api";

type AudioBriefingPresetApplyModalProps = {
  open: boolean;
  loading: boolean;
  error: string | null;
  presets: AudioBriefingPreset[];
  selectedPresetID: string | null;
  onClose: () => void;
  onRefresh: () => void;
  onSelectPreset: (presetID: string) => void;
  onApplyPreset: (preset: AudioBriefingPreset) => void;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export default function AudioBriefingPresetApplyModal({
  open,
  loading,
  error,
  presets,
  selectedPresetID,
  onClose,
  onRefresh,
  onSelectPreset,
  onApplyPreset,
}: AudioBriefingPresetApplyModalProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");

  const filteredPresets = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return presets;
    return presets.filter((preset) =>
      [preset.name, preset.default_persona_mode, preset.default_persona, preset.conversation_mode, ...preset.voices.flatMap((voice) => [
        voice.persona,
        voice.tts_provider,
        voice.tts_model,
        voice.voice_model,
        voice.voice_style,
        voice.provider_voice_label ?? "",
        voice.provider_voice_description ?? "",
      ])]
        .join(" ")
        .toLowerCase()
        .includes(q),
    );
  }, [presets, query]);

  useEffect(() => {
    if (open) {
      setQuery("");
    }
  }, [open]);

  const selectedPreset = useMemo(() => {
    if (!filteredPresets.length) return null;
    return filteredPresets.find((preset) => preset.id === selectedPresetID) ?? filteredPresets[0];
  }, [filteredPresets, selectedPresetID]);

  useEffect(() => {
    if (!open || selectedPreset) return;
    if (filteredPresets[0]) {
      onSelectPreset(filteredPresets[0].id);
    }
  }, [filteredPresets, onSelectPreset, open, selectedPreset]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.presetApplyTitle")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.presetApplySubtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={onRefresh}
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              {t("common.refresh")}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              aria-label={t("common.close")}
            >
              <X className="size-4" />
            </button>
          </div>
        </div>

        <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.9fr)]">
          <div className="min-h-0 overflow-auto border-b border-[var(--color-editorial-line)] px-5 py-5 lg:border-b-0 lg:border-r">
            <div className="flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
              <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t("settings.audioBriefing.presetSearchPlaceholder")}
                className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
              />
            </div>

            {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}

            <div className="mt-4 space-y-2">
              {loading ? (
                <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white px-4 py-6 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                  {t("common.loading")}
                </div>
              ) : filteredPresets.length === 0 ? (
                <div className="rounded-[18px] border border-dashed border-[var(--color-editorial-line)] bg-white/80 px-4 py-6 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                  {presets.length === 0 ? t("settings.audioBriefing.presetEmpty") : t("settings.audioBriefing.presetNoResults")}
                </div>
              ) : (
                filteredPresets.map((preset) => {
                  const active = preset.id === selectedPreset?.id;
                  return (
                    <button
                      key={preset.id}
                      type="button"
                      onClick={() => onSelectPreset(preset.id)}
                      className={joinClassNames(
                        "w-full rounded-[18px] border px-4 py-4 text-left transition",
                        active
                          ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-panel-strong)] shadow-[var(--shadow-card)]"
                          : "border-[var(--color-editorial-line)] bg-white hover:bg-[var(--color-editorial-panel-strong)]",
                      )}
                    >
                      <div className="flex flex-wrap items-start justify-between gap-2">
                        <div>
                          <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{preset.name}</div>
                          <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">
                            {t(`settings.audioBriefing.conversationMode.${preset.conversation_mode === "duo" ? "duo" : "single"}`)}
                          </div>
                        </div>
                        <span className="rounded-full border border-[var(--color-editorial-line)] bg-white px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-ink-soft)]">
                          {preset.default_persona_mode === "random"
                            ? t("settings.personaMode.random")
                            : t(`settings.navigator.persona.${preset.default_persona}`, preset.default_persona)}
                        </span>
                      </div>
                      <div className="mt-3 flex flex-wrap gap-2 text-[12px] text-[var(--color-editorial-ink-soft)]">
                        <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1">
                          {`${preset.voices.filter((voice) => voice.voice_model.trim()).length}/${preset.voices.length} ${t("settings.audioBriefing.summary.configured")}`}
                        </span>
                        <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1">
                          {preset.updated_at ? new Date(preset.updated_at).toLocaleString() : t("common.unknown")}
                        </span>
                      </div>
                    </button>
                  );
                })
              )}
            </div>
          </div>

          <div className="min-h-0 overflow-auto px-5 py-5">
            {selectedPreset ? (
              <div className="space-y-5">
                <div>
                  <div className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                    {t("settings.audioBriefing.presetSelected")}
                  </div>
                  <h3 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedPreset.name}</h3>
                  <p className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.presetApplyHelp")}</p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.audioBriefing.defaultPersona")}
                    </div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                      {selectedPreset.default_persona_mode === "random"
                        ? t("settings.personaMode.random")
                        : t(`settings.navigator.persona.${selectedPreset.default_persona}`, selectedPreset.default_persona)}
                    </div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.audioBriefing.conversationMode")}
                    </div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                      {t(`settings.audioBriefing.conversationMode.${selectedPreset.conversation_mode === "duo" ? "duo" : "single"}`)}
                    </div>
                  </div>
                </div>

                <div className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                  <div className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                    {t("settings.audioBriefing.presetVoices")}
                  </div>
                  <div className="mt-3 space-y-2">
                    {normalizeAudioBriefingPresetVoices(selectedPreset.voices).map((voice) => (
                      <div key={voice.persona} className="rounded-[16px] border border-[var(--color-editorial-line)] bg-white px-4 py-3">
                        <div className="flex flex-wrap items-start justify-between gap-2">
                          <div>
                            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">
                              {t(`settings.navigator.persona.${voice.persona}`, voice.persona)}
                            </div>
                            <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">
                              {formatAudioBriefingPresetVoiceLabel(voice, t)}
                            </div>
                          </div>
                          <div className="text-[12px] text-[var(--color-editorial-ink-soft)]">
                            {formatAudioBriefingPresetVoiceDetail(voice, t)}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => onApplyPreset(selectedPreset)}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                  >
                    {t("settings.audioBriefing.presetApplyConfirm")}
                  </button>
                </div>
              </div>
            ) : (
              <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.presetApplyEmpty")}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
