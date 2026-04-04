"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Search } from "lucide-react";
import { api, GeminiTTSVoiceCatalogEntry, GeminiTTSVoicesResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";

export default function GeminiTTSVoicesPage() {
  const { t } = useI18n();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [data, setData] = useState<GeminiTTSVoicesResponse | null>(null);
  const [selectedVoiceName, setSelectedVoiceName] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const next = await api.getGeminiTTSVoices();
      setData(next);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const filteredVoices = useMemo(() => {
    const q = query.trim().toLowerCase();
    const voices = data?.voices ?? [];
    if (!q) return voices;
    return voices.filter((voice) =>
      [voice.voice_name, voice.label, voice.tone, voice.description]
        .join(" ")
        .toLowerCase()
        .includes(q)
    );
  }, [data?.voices, query]);

  const selectedVoice = useMemo<GeminiTTSVoiceCatalogEntry | null>(() => {
    const voices = data?.voices ?? [];
    if (!selectedVoiceName) return voices[0] ?? null;
    return voices.find((voice) => voice.voice_name === selectedVoiceName) ?? voices[0] ?? null;
  }, [data?.voices, selectedVoiceName]);

  return (
    <PageTransition>
      <div className="space-y-6">
        <PageHeader
          title={t("geminiTTS.title")}
          description={t("geminiTTS.subtitle")}
        />

        <section className="grid gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(320px,0.65fr)]">
          <div className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5 shadow-[0_18px_48px_rgba(35,24,12,0.08)]">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("geminiTTS.searchTitle")}</div>
                <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("geminiTTS.searchDescription")}</p>
              </div>
              <div className="text-sm text-[var(--color-editorial-ink-soft)]">
                {`${t("geminiTTS.voiceCount")}: ${data?.voices.length ?? 0}`}
              </div>
            </div>

            <div className="mt-5 flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
              <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t("geminiTTS.search")}
                className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
              />
            </div>

            {error ? <p className="mt-4 text-sm text-red-600">{error}</p> : null}

            <div className="mt-5 overflow-hidden rounded-[24px] border border-[var(--color-editorial-line)] bg-white">
              <div className="overflow-x-auto">
                <table className="min-w-[760px] divide-y divide-[var(--color-editorial-line)] text-sm">
                  <thead className="bg-[var(--color-editorial-panel-strong)]">
                    <tr className="text-left text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                      <th className="px-4 py-3">{t("geminiTTS.table.voice")}</th>
                      <th className="px-4 py-3">{t("geminiTTS.table.tone")}</th>
                      <th className="px-4 py-3">{t("geminiTTS.table.description")}</th>
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
                          {t("geminiTTS.noVoices")}
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
                          <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{voice.tone || "—"}</td>
                          <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{voice.description || "—"}</td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <aside className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5 shadow-[0_18px_48px_rgba(35,24,12,0.08)]">
            {selectedVoice ? (
              <div className="space-y-5">
                <div>
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("geminiTTS.detail.title")}</div>
                  <h2 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.label || selectedVoice.voice_name}</h2>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{selectedVoice.description || t("geminiTTS.detail.noDescription")}</p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("geminiTTS.table.voice")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.voice_name}</div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("geminiTTS.table.tone")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.tone || "—"}</div>
                  </div>
                </div>

                {selectedVoice.sample_audio_path ? (
                  <div>
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("geminiTTS.preview")}</div>
                    <div className="mt-3 rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                      <audio controls preload="none" className="w-full" src={selectedVoice.sample_audio_path} />
                    </div>
                  </div>
                ) : null}
              </div>
            ) : (
              <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("geminiTTS.detail.emptySelection")}
              </div>
            )}
          </aside>
        </section>
      </div>
    </PageTransition>
  );
}
