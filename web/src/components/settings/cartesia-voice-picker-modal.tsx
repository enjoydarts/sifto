"use client";

import { useEffect, useMemo, useState } from "react";
import { RefreshCw, Search, X } from "lucide-react";
import { api, type CartesiaVoiceCatalogEntry } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

type CartesiaVoicePickerModalProps = {
  open: boolean;
  loading: boolean;
  error: string | null;
  voices: CartesiaVoiceCatalogEntry[];
  currentVoiceID: string;
  onClose: () => void;
  onRefresh: () => Promise<void> | void;
  onSelect: (selection: { voice_id: string; label: string; description: string }) => void;
};

export default function CartesiaVoicePickerModal({
  open,
  loading,
  error,
  voices,
  currentVoiceID,
  onClose,
  onRefresh,
  onSelect,
}: CartesiaVoicePickerModalProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [selectedVoiceID, setSelectedVoiceID] = useState<string | null>(null);
  const [previewURL, setPreviewURL] = useState<string | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const filteredVoices = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return voices;
    return voices.filter((voice) =>
      [voice.voice_id, voice.name, voice.description, voice.language]
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
      filteredVoices[0] ??
      null
    );
  }, [currentVoiceID, filteredVoices, selectedVoiceID, voices]);

  useEffect(() => {
    setPreviewError(null);
    setPreviewLoading(false);
    setPreviewURL((current) => {
      if (current) URL.revokeObjectURL(current);
      return null;
    });
  }, [selectedVoice?.voice_id]);

  useEffect(() => {
    return () => {
      if (previewURL) URL.revokeObjectURL(previewURL);
    };
  }, [previewURL]);

  const loadPreview = async () => {
    if (!selectedVoice?.voice_id) return;
    setPreviewLoading(true);
    setPreviewError(null);
    try {
      const blob = await api.getCartesiaVoicePreview(selectedVoice.voice_id);
      setPreviewURL((current) => {
        if (current) URL.revokeObjectURL(current);
        return URL.createObjectURL(blob);
      });
    } catch (err) {
      setPreviewError(String(err));
    } finally {
      setPreviewLoading(false);
    }
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.cartesiaPicker.title")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.cartesiaPicker.subtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => void onRefresh()}
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              <RefreshCw className="size-4" />
              {t("settings.cartesiaPicker.refresh")}
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
              placeholder={t("settings.cartesiaPicker.search")}
              className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
            />
          </div>
          {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}
        </div>

        <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_minmax(300px,0.85fr)]">
          <div className="min-h-0 overflow-auto border-b border-[var(--color-editorial-line)] px-5 py-5 lg:border-b-0 lg:border-r">
            {loading ? (
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white px-5 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                {t("common.loading")}
              </div>
            ) : filteredVoices.length === 0 ? (
              <div className="rounded-[18px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {voices.length === 0 ? t("settings.cartesiaPicker.empty") : t("settings.cartesiaPicker.noResults")}
              </div>
            ) : (
              <div className="space-y-2">
                {filteredVoices.map((voice) => {
                  const active = selectedVoice?.voice_id === voice.voice_id;
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
                      <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{voice.name || voice.voice_id}</div>
                      <div className="mt-1 break-all text-xs text-[var(--color-editorial-ink-soft)]">{voice.voice_id}</div>
                      <p className="mt-3 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">
                        {voice.description || t("settings.cartesiaPicker.noDescription")}
                      </p>
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          <div className="min-h-0 overflow-auto px-5 py-5">
            {selectedVoice ? (
              <div className="space-y-5">
                <div>
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.cartesiaPicker.selected")}</div>
                  <h3 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.name || selectedVoice.voice_id}</h3>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                    {selectedVoice.description || t("settings.cartesiaPicker.noDescription")}
                  </p>
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.cartesiaPicker.voiceId")}</div>
                    <div className="mt-2 break-all text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.voice_id}</div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.cartesiaPicker.language")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.language || "ja"}</div>
                  </div>
                </div>
                <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.cartesiaPicker.preview")}</div>
                    <button
                      type="button"
                      onClick={() => void loadPreview()}
                      disabled={previewLoading}
                      className="inline-flex min-h-9 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      {previewLoading ? t("common.loading") : t("settings.cartesiaPicker.loadPreview")}
                    </button>
                  </div>
                  {previewURL ? <audio controls preload="none" className="mt-3 w-full" src={previewURL} /> : null}
                  {previewError ? <p className="mt-3 text-xs leading-5 text-red-600">{previewError}</p> : null}
                </div>
                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => {
                      onSelect({
                        voice_id: selectedVoice.voice_id,
                        label: selectedVoice.name || selectedVoice.voice_id,
                        description: selectedVoice.description,
                      });
                      onClose();
                    }}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                  >
                    {t("settings.cartesiaPicker.select")}
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
