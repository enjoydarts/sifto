"use client";

import { useMemo, useState } from "react";
import { RefreshCw, Search, X } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import type { AzureSpeechVoiceCatalogEntry } from "@/lib/api";

type AzureSpeechVoicePickerModalProps = {
  open: boolean;
  loading: boolean;
  error: string | null;
  voices: AzureSpeechVoiceCatalogEntry[];
  currentVoiceID: string;
  onClose: () => void;
  onRefresh: () => Promise<void> | void;
  onSelect: (selection: { voice_id: string; label: string; description: string }) => void;
};

export default function AzureSpeechVoicePickerModal({
  open,
  loading,
  error,
  voices,
  currentVoiceID,
  onClose,
  onRefresh,
  onSelect,
}: AzureSpeechVoicePickerModalProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [selectedVoiceID, setSelectedVoiceID] = useState<string | null>(null);

  const filteredVoices = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return voices;
    return voices.filter((voice) =>
      [voice.voice_id, voice.label, voice.description, voice.locale, voice.gender, voice.local_name, (voice.styles ?? []).join(" ")]
        .join(" ")
        .toLowerCase()
        .includes(q),
    );
  }, [query, voices]);

  const selectedVoice = useMemo(() => {
    const activeVoiceID = selectedVoiceID ?? currentVoiceID;
    return (
      filteredVoices.find((voice) => voice.voice_id === activeVoiceID) ??
      voices.find((voice) => voice.voice_id === activeVoiceID) ??
      null
    );
  }, [currentVoiceID, filteredVoices, selectedVoiceID, voices]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.azureSpeechPickerTitle")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.azureSpeechPickerSubtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => void onRefresh()}
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              <RefreshCw className="size-4" />
              {t("settings.audioBriefing.refreshAzureSpeechCatalog")}
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

        <div className="border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div className="flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
            <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("settings.audioBriefing.azureSpeechPickerSearch")}
              className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
            />
          </div>
          {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}
        </div>

        <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.85fr)]">
          <div className="min-h-0 overflow-auto border-b border-[var(--color-editorial-line)] lg:border-b-0 lg:border-r">
            <div className="overflow-x-auto">
              <table className="min-w-[860px] divide-y divide-[var(--color-editorial-line)] text-sm">
                <thead className="bg-[var(--color-editorial-panel-strong)]">
                  <tr className="text-left text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                    <th className="px-4 py-3">{t("settings.audioBriefing.azureSpeechVoiceTable.voice")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.azureSpeechVoiceTable.locale")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.azureSpeechVoiceTable.gender")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.azureSpeechVoiceTable.styles")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--color-editorial-line)] bg-white">
                  {loading ? (
                    <tr>
                      <td colSpan={4} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("common.loading")}
                      </td>
                    </tr>
                  ) : filteredVoices.length === 0 ? (
                    <tr>
                      <td colSpan={4} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("settings.audioBriefing.azureSpeechPickerNoResults")}
                      </td>
                    </tr>
                  ) : (
                    filteredVoices.map((voice) => (
                      <tr
                        key={voice.voice_id}
                        className={`cursor-pointer transition hover:bg-[var(--color-editorial-panel)] ${selectedVoice?.voice_id === voice.voice_id ? "bg-[var(--color-editorial-panel)]" : ""}`}
                        onClick={() => setSelectedVoiceID(voice.voice_id)}
                      >
                        <td className="px-4 py-3">
                          <div className="font-medium text-[var(--color-editorial-ink)]">{voice.label || voice.voice_id}</div>
                          <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{voice.voice_id}</div>
                        </td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink)]">{voice.locale || "—"}</td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{voice.gender || "—"}</td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{voice.styles?.length ? voice.styles.join(", ") : "—"}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <div className="min-h-0 overflow-auto px-5 py-5">
            {selectedVoice ? (
              <div className="space-y-5">
                <div>
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.azureSpeechPickerSelected")}</div>
                  <h3 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.label || selectedVoice.voice_id}</h3>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                    {selectedVoice.description || t("settings.audioBriefing.azureSpeechPickerNoDescription")}
                  </p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.azureSpeechVoiceTable.locale")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.locale || "—"}</div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.azureSpeechVoiceTable.gender")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.gender || "—"}</div>
                  </div>
                </div>

                <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.azureSpeechVoiceTable.styles")}</div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {selectedVoice.styles?.length ? (
                      selectedVoice.styles.map((style) => (
                        <span
                          key={`${selectedVoice.voice_id}-${style}`}
                          className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-xs text-[var(--color-editorial-ink-soft)]"
                        >
                          {style}
                        </span>
                      ))
                    ) : (
                      <span className="text-sm text-[var(--color-editorial-ink-soft)]">—</span>
                    )}
                  </div>
                </div>

                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => {
                      onSelect({
                        voice_id: selectedVoice.voice_id,
                        label: selectedVoice.label || selectedVoice.voice_id,
                        description: selectedVoice.description,
                      });
                      onClose();
                    }}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                  >
                    {t("settings.audioBriefing.azureSpeechPickerSelect")}
                  </button>
                </div>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}
