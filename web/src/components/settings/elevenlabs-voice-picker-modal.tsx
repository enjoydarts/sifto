"use client";

import { useEffect, useMemo, useState } from "react";
import { RefreshCw, Search, X } from "lucide-react";
import { ElevenLabsVoiceCatalogEntry } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

type ElevenLabsVoicePickerModalProps = {
  open: boolean;
  loading: boolean;
  error: string | null;
  voices: ElevenLabsVoiceCatalogEntry[];
  currentVoiceID: string;
  onClose: () => void;
  onRefresh: () => Promise<void> | void;
  onSelect: (selection: { voice_id: string; label: string; description: string }) => void;
};

function voiceLabels(voice: ElevenLabsVoiceCatalogEntry): string[] {
  const labels = voice.labels;
  if (!labels || typeof labels !== "object") return [];
  return Object.entries(labels)
    .flatMap(([key, value]) => {
      if (value == null || value === "") return [];
      return [`${key}: ${String(value)}`];
    })
    .slice(0, 4);
}

export default function ElevenLabsVoicePickerModal({
  open,
  loading,
  error,
  voices,
  currentVoiceID,
  onClose,
  onRefresh,
  onSelect,
}: ElevenLabsVoicePickerModalProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [selectedVoiceID, setSelectedVoiceID] = useState<string | null>(null);

  const japaneseVoices = useMemo(
    () =>
      voices.filter((voice) => {
        const joinedMeta = [
          ...(voice.languages ?? []),
          voice.voice_id,
          voice.name,
          voice.description,
          voice.category ?? "",
          ...Object.values(voice.labels ?? {}).map((value) => String(value ?? "")),
        ]
          .join(" ")
          .toLowerCase();
        return (
          joinedMeta.includes("ja") ||
          joinedMeta.includes("japanese") ||
          joinedMeta.includes("日本語") ||
          joinedMeta.includes("日本")
        );
      }),
    [voices]
  );

  const filteredVoices = useMemo(() => {
    const q = query.trim().toLowerCase();
    const base = japaneseVoices.length > 0 ? japaneseVoices : voices;
    if (!q) return base;
    return base.filter((voice) =>
      [
        voice.voice_id,
        voice.name,
        voice.description,
        voice.category ?? "",
        (voice.languages ?? []).join(" "),
        voiceLabels(voice).join(" "),
      ]
        .join(" ")
        .toLowerCase()
        .includes(q)
    );
  }, [japaneseVoices, query, voices]);

  const selectedVoice = useMemo(() => {
    const activeVoiceID = selectedVoiceID ?? currentVoiceID;
    return (
      filteredVoices.find((voice) => voice.voice_id === activeVoiceID) ??
      voices.find((voice) => voice.voice_id === activeVoiceID) ??
      null
    );
  }, [currentVoiceID, filteredVoices, selectedVoiceID, voices]);

  const pinnedCurrentSelection = useMemo(() => {
    if (!currentVoiceID.trim()) return null;
    if (voices.some((voice) => voice.voice_id === currentVoiceID)) return null;
    return {
      voice_id: currentVoiceID,
      name: currentVoiceID,
      description: t("settings.elevenlabsPicker.currentSelectionFallback"),
      category: null as string | null,
      preview_url: "",
      labels: undefined,
    } satisfies ElevenLabsVoiceCatalogEntry;
  }, [currentVoiceID, t, voices]);

  useEffect(() => {
    if (!open) {
      setSelectedVoiceID(null);
      return;
    }
    setSelectedVoiceID(currentVoiceID || null);
  }, [currentVoiceID, open]);

  useEffect(() => {
    if (!open) return;
    if (currentVoiceID.trim() && !voices.some((voice) => voice.voice_id === currentVoiceID)) return;
    if (selectedVoiceID && voices.some((voice) => voice.voice_id === selectedVoiceID)) return;
    if (filteredVoices[0]) {
      setSelectedVoiceID(filteredVoices[0].voice_id);
    }
  }, [filteredVoices, open, selectedVoiceID, voices]);

  if (!open) return null;

  const displayedVoice = selectedVoice ?? pinnedCurrentSelection ?? filteredVoices[0] ?? voices[0] ?? null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.elevenlabsPicker.title")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.elevenlabsPicker.subtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => void onRefresh()}
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              <RefreshCw className="size-4" />
              {t("settings.elevenlabsPicker.refresh")}
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
              placeholder={t("settings.elevenlabsPicker.search")}
              className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
            />
          </div>
          {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}
        </div>

        <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.9fr)]">
          <div className="min-h-0 overflow-auto border-b border-[var(--color-editorial-line)] px-5 py-5 lg:border-b-0 lg:border-r">
            {loading ? (
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-white px-5 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                {t("common.loading")}
              </div>
            ) : filteredVoices.length === 0 ? (
              <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {voices.length === 0 ? t("settings.elevenlabsPicker.empty") : t("settings.elevenlabsPicker.noResults")}
              </div>
            ) : (
              <div className="space-y-2">
                {filteredVoices.map((voice) => {
                  const active = (displayedVoice?.voice_id ?? "") === voice.voice_id;
                  return (
                    <button
                      key={voice.voice_id}
                      type="button"
                      onClick={() => setSelectedVoiceID(voice.voice_id)}
                      className={`w-full rounded-[18px] border px-4 py-4 text-left transition ${
                        active
                          ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-panel-strong)]"
                          : "border-[var(--color-editorial-line)] bg-white hover:bg-[var(--color-editorial-panel-strong)]"
                      }`}
                    >
                      <div className="flex flex-wrap items-start justify-between gap-2">
                        <div className="min-w-0">
                          <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{voice.name || voice.voice_id}</div>
                          <div className="mt-1 break-all text-xs text-[var(--color-editorial-ink-soft)]">{voice.voice_id}</div>
                        </div>
                        <div className="flex flex-wrap gap-2 text-[11px] text-[var(--color-editorial-ink-soft)]">
                          {voice.category ? (
                            <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-1">
                              {voice.category}
                            </span>
                          ) : null}
                        </div>
                      </div>
                      <p className="mt-3 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">
                        {voice.description || t("settings.elevenlabsPicker.noDescription")}
                      </p>
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          <div className="min-h-0 overflow-auto px-5 py-5">
            {displayedVoice ? (
              <div className="space-y-5">
                <div>
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                    {t("settings.elevenlabsPicker.selected")}
                  </div>
                  <h3 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{displayedVoice.name || displayedVoice.voice_id}</h3>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                    {displayedVoice.description || t("settings.elevenlabsPicker.noDescription")}
                  </p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.elevenlabsPicker.voiceId")}
                    </div>
                    <div className="mt-2 break-all text-sm font-semibold text-[var(--color-editorial-ink)]">
                      {displayedVoice.voice_id}
                    </div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.elevenlabsPicker.category")}
                    </div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                      {displayedVoice.category || "—"}
                    </div>
                  </div>
                </div>

                {voiceLabels(displayedVoice).length > 0 ? (
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.elevenlabsPicker.labels")}
                    </div>
                    <div className="mt-3 flex flex-wrap gap-2">
                      {voiceLabels(displayedVoice).map((label) => (
                        <span
                          key={`${displayedVoice.voice_id}-${label}`}
                          className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-xs text-[var(--color-editorial-ink-soft)]"
                        >
                          {label}
                        </span>
                      ))}
                    </div>
                  </div>
                ) : null}

                {displayedVoice.preview_url ? (
                  <div>
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.elevenlabsPicker.preview")}
                    </div>
                    <div className="mt-3 rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                      <audio controls preload="none" className="w-full" src={displayedVoice.preview_url} />
                    </div>
                  </div>
                ) : null}

                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => {
                      onSelect({
                        voice_id: displayedVoice.voice_id,
                        label: displayedVoice.name || displayedVoice.voice_id,
                        description: displayedVoice.description || t("settings.elevenlabsPicker.noDescription"),
                      });
                      onClose();
                    }}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                  >
                    {t("settings.elevenlabsPicker.select")}
                  </button>
                </div>
              </div>
            ) : (
              <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("settings.elevenlabsPicker.currentSelectionFallback")}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
