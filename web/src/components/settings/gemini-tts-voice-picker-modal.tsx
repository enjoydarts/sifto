"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { RefreshCw, Search, X } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import type { GeminiTTSVoiceCatalogEntry } from "@/lib/api";

type GeminiTTSVoicePickerModalProps = {
  open: boolean;
  loading: boolean;
  error: string | null;
  voices: GeminiTTSVoiceCatalogEntry[];
  currentVoiceName: string;
  onClose: () => void;
  onRefresh: () => Promise<void> | void;
  onSelect: (selection: { voice_name: string }) => void;
};

export default function GeminiTTSVoicePickerModal({
  open,
  loading,
  error,
  voices,
  currentVoiceName,
  onClose,
  onRefresh,
  onSelect,
}: GeminiTTSVoicePickerModalProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [selectedVoiceName, setSelectedVoiceName] = useState<string | null>(null);

  const filteredVoices = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return voices;
    return voices.filter((voice) =>
      [voice.voice_name, voice.label, voice.tone, voice.description].join(" ").toLowerCase().includes(q),
    );
  }, [query, voices]);

  const selectedVoice = useMemo(() => {
    const activeVoiceName = selectedVoiceName ?? currentVoiceName;
    return (
      filteredVoices.find((voice) => voice.voice_name === activeVoiceName) ??
      voices.find((voice) => voice.voice_name === activeVoiceName) ??
      null
    );
  }, [currentVoiceName, filteredVoices, selectedVoiceName, voices]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.geminiTTSPickerTitle")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.geminiTTSPickerSubtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <Link
              href="/gemini-tts-voices"
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              {t("settings.audioBriefing.openGeminiTTSVoices")}
            </Link>
            <button
              type="button"
              onClick={() => void onRefresh()}
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              <RefreshCw className="size-4" />
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

        <div className="border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div className="flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
            <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("settings.audioBriefing.geminiTTSPickerSearch")}
              className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
            />
          </div>
          {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}
        </div>

        <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.85fr)]">
          <div className="min-h-0 overflow-auto border-b border-[var(--color-editorial-line)] lg:border-b-0 lg:border-r">
            <div className="overflow-x-auto">
              <table className="min-w-[760px] divide-y divide-[var(--color-editorial-line)] text-sm">
                <thead className="bg-[var(--color-editorial-panel-strong)]">
                  <tr className="text-left text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                    <th className="px-4 py-3">{t("settings.audioBriefing.geminiTTSVoiceTable.voice")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.geminiTTSVoiceTable.tone")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.geminiTTSVoiceTable.description")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--color-editorial-line)] bg-white">
                  {loading ? (
                    <tr>
                      <td colSpan={3} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("common.loading")}
                      </td>
                    </tr>
                  ) : filteredVoices.length === 0 ? (
                    <tr>
                      <td colSpan={3} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("settings.audioBriefing.geminiTTSPickerNoResults")}
                      </td>
                    </tr>
                  ) : (
                    filteredVoices.map((voice) => (
                      <tr
                        key={voice.voice_name}
                        className={`cursor-pointer transition hover:bg-[var(--color-editorial-panel)] ${selectedVoice?.voice_name === voice.voice_name ? "bg-[var(--color-editorial-panel)]" : ""}`}
                        onClick={() => setSelectedVoiceName(voice.voice_name)}
                      >
                        <td className="px-4 py-3">
                          <div className="font-medium text-[var(--color-editorial-ink)]">{voice.label || voice.voice_name}</div>
                          <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{voice.voice_name}</div>
                        </td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink)]">{voice.tone || "—"}</td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{voice.description || "—"}</td>
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
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.geminiTTSPickerSelected")}</div>
                  <h3 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.label || selectedVoice.voice_name}</h3>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                    {selectedVoice.description || t("settings.audioBriefing.geminiTTSPickerNoDescription")}
                  </p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.geminiTTSVoiceTable.voice")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.voice_name}</div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.geminiTTSVoiceTable.tone")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.tone || "—"}</div>
                  </div>
                </div>

                {selectedVoice.sample_audio_path ? (
                  <div>
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.geminiTTSPreview")}</div>
                    <div className="mt-3 rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                      <audio controls preload="none" className="w-full" src={selectedVoice.sample_audio_path} />
                    </div>
                  </div>
                ) : null}

                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => {
                      onSelect({ voice_name: selectedVoice.voice_name });
                      onClose();
                    }}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                  >
                    {t("settings.audioBriefing.geminiTTSPickerSelect")}
                  </button>
                </div>
              </div>
            ) : (
              <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.geminiTTSPickerEmptySelection")}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
